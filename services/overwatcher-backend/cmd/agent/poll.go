package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
)

// Poller pulls one intent at a time from the coordinator and feeds it to the
// runner. The loop is single-threaded by design — Phase 3 is one VM, one
// stack, one deploy at a time. Concurrent deploys are Phase 4.
type Poller struct {
	httpClient *http.Client
	cfg        AgentConfig
	runner     *Runner
	nextURL    string
}

func NewPoller(cfg AgentConfig, runner *Runner) (*Poller, error) {
	base, err := url.Parse(cfg.CoordinatorURL)
	if err != nil {
		return nil, fmt.Errorf("invalid coordinator_url: %w", err)
	}
	next, _ := url.Parse("/api/v1/deploy/next")

	return &Poller{
		httpClient: &http.Client{Timeout: cfg.PollTimeout},
		cfg:        cfg,
		runner:     runner,
		nextURL:    base.ResolveReference(next).String(),
	}, nil
}

// Run loops until ctx is cancelled. Each iteration: long-poll for an intent,
// run it, post the result, repeat. Network errors trigger exponential backoff
// up to 30s; everything else logs and continues.
func (p *Poller) Run(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		intent, err := p.fetchNext(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("poll error, backing off", "error", err, "backoff", backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = time.Second

		if intent == nil {
			// 204 — re-poll immediately.
			continue
		}

		p.handleIntent(ctx, intent)
	}
}

// fetchNext returns nil intent on 204 (long-poll timeout, no work). Returns
// an error on transport failure or unexpected status.
func (p *Poller) fetchNext(ctx context.Context) (*dto.DeployIntentResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.nextURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.SharedSecret)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil, nil
	case http.StatusOK:
		var intent dto.DeployIntentResponse
		if err := json.NewDecoder(resp.Body).Decode(&intent); err != nil {
			return nil, fmt.Errorf("decode intent: %w", err)
		}
		return &intent, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func (p *Poller) handleIntent(ctx context.Context, intent *dto.DeployIntentResponse) {
	slog.Info("intent received",
		"intent_id", intent.ID,
		"repo", intent.Repo,
		"stack", intent.Stack,
		"sha", intent.SHA,
	)

	runErr := p.runner.Run(ctx, intent)
	success := runErr == nil
	errMsg := ""
	if !success {
		errMsg = runErr.Error()
		slog.Error("deploy failed", "intent_id", intent.ID, "error", runErr)
	} else {
		slog.Info("deploy succeeded", "intent_id", intent.ID)
	}

	if err := p.postResult(ctx, intent.ID, success, errMsg); err != nil {
		// Result reporting is best-effort: if it fails the intent stays in
		// the coordinator's in-flight map until an operator drains it.
		// Phase 4 will add proper retry/timeout.
		slog.Error("post result failed", "intent_id", intent.ID, "error", err)
	}
}

func (p *Poller) postResult(ctx context.Context, intentID string, success bool, errMsg string) error {
	state := "success"
	if !success {
		state = "failure"
	}
	body, err := json.Marshal(dto.DeployResultRequest{State: state, Error: errMsg})
	if err != nil {
		return err
	}

	resultURL := strings.TrimRight(p.cfg.CoordinatorURL, "/") + "/api/v1/deploy/" + url.PathEscape(intentID) + "/result"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resultURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.SharedSecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return errors.New("coordinator " + resp.Status + ": " + strings.TrimSpace(string(body)))
	}
	return nil
}
