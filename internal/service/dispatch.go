package service

import (
	"context"
	"log/slog"
	"strings"

	gh "github.com/google/go-github/v84/github"

	internalgithub "github.com/lwlee2608/overwatcher/internal/github"
)

// statusUpdater is the slice of GitHub API DispatchService needs. It exists so
// tests can substitute a fake without spinning up a real installation client.
type statusUpdater interface {
	UpdateDeploymentStatus(ctx context.Context, installationID int64, owner, repo string, deploymentID int64, state, description string) error
}

// ghStatusUpdater is the production adapter that wraps internalgithub.Client.
type ghStatusUpdater struct {
	client *internalgithub.Client
}

func (g *ghStatusUpdater) UpdateDeploymentStatus(ctx context.Context, installationID int64, owner, repo string, deploymentID int64, state, description string) error {
	client, err := g.client.GetInstallationClient(ctx, installationID)
	if err != nil {
		return err
	}
	_, _, err = client.Repositories.CreateDeploymentStatus(ctx, owner, repo, deploymentID, &gh.DeploymentStatusRequest{
		State:       gh.Ptr(state),
		Description: gh.Ptr(description),
	})
	return err
}

// DispatchService consumes intents from the IntentStore and reports their
// outcome back to GitHub. It is the counterpart to WebhookService, which
// produces intents.
type DispatchService struct {
	store   *IntentStore
	updater statusUpdater
}

func NewDispatchService(ghClient *internalgithub.Client, store *IntentStore) *DispatchService {
	return &DispatchService{
		store:   store,
		updater: &ghStatusUpdater{client: ghClient},
	}
}

// Next blocks until an intent is available or ctx is cancelled. On success it
// flips the GitHub Deployment to in_progress (best-effort — failures here are
// logged but the intent is still returned, matching Phase 2's source-of-truth
// principle) and returns the intent.
func (d *DispatchService) Next(ctx context.Context) (*DeployIntent, error) {
	intent, err := d.store.TakeNext(ctx)
	if err != nil {
		return nil, err
	}

	owner, repoName := splitRepo(intent.Repo)
	if err := d.updater.UpdateDeploymentStatus(ctx, intent.InstallationID, owner, repoName, intent.DeploymentID, "in_progress", "Deploy in progress"); err != nil {
		slog.Warn("Failed to mark deployment in_progress; intent still dispatched",
			"intent_id", intent.ID,
			"deployment_id", intent.DeploymentID,
			"error", err,
		)
	}

	slog.Info("Intent dispatched to agent",
		"intent_id", intent.ID,
		"repo", intent.Repo,
		"stack", intent.Stack,
		"sha", intent.SHA,
	)
	return intent, nil
}

// Report records an agent's deploy result, removes the intent from the
// in-flight map, and updates the GitHub Deployment to success/failure. Returns
// false if the id is unknown.
func (d *DispatchService) Report(ctx context.Context, id string, success bool, errMsg string) bool {
	intent, ok := d.store.Complete(id, success)
	if !ok {
		return false
	}

	state := "success"
	description := "Deploy succeeded"
	if !success {
		state = "failure"
		description = "Deploy failed"
		if errMsg != "" {
			description = "Deploy failed: " + errMsg
		}
	}

	owner, repoName := splitRepo(intent.Repo)
	if err := d.updater.UpdateDeploymentStatus(ctx, intent.InstallationID, owner, repoName, intent.DeploymentID, state, description); err != nil {
		slog.Warn("Failed to update deployment status",
			"intent_id", id,
			"state", state,
			"error", err,
		)
	}

	slog.Info("Intent completed",
		"intent_id", id,
		"repo", intent.Repo,
		"stack", intent.Stack,
		"state", state,
	)
	return true
}

func splitRepo(full string) (owner, repo string) {
	if i := strings.IndexByte(full, '/'); i >= 0 {
		return full[:i], full[i+1:]
	}
	return "", full
}
