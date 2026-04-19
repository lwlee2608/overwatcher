package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	gh "github.com/google/go-github/v84/github"

	internalgithub "github.com/lwlee2608/overwatcher/internal/github"
	"github.com/lwlee2608/overwatcher/internal/service/eventlog"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
	"github.com/lwlee2608/overwatcher/internal/service/mapping"
)

type Service struct {
	ghClient *internalgithub.Client
	mapping  *mapping.Service
	store    intent.Store
	eventLog *eventlog.Service
}

func New(ghClient *internalgithub.Client, m *mapping.Service, store intent.Store, el *eventlog.Service) *Service {
	return &Service{ghClient: ghClient, mapping: m, store: store, eventLog: el}
}

func (s *Service) HandleEvent(ctx context.Context, eventType string, deliveryID string, payload []byte) error {
	repo, sender, summary := extractEventInfo(eventType, payload)

	if err := s.eventLog.Record(ctx, deliveryID, eventType, repo, sender, summary); err != nil {
		slog.Error("Failed to record event log", "delivery_id", deliveryID, "error", err)
	}

	event, err := gh.ParseWebHook(eventType, payload)
	if err != nil {
		slog.Error("Failed to parse webhook payload", "event", eventType, "delivery_id", deliveryID, "error", err)
		return err
	}

	switch eventType {
	case internalgithub.EventPush:
		s.handlePush(ctx, event.(*gh.PushEvent), deliveryID)
	default:
		slog.Info("Unhandled webhook event", "event", eventType, "delivery_id", deliveryID)
	}

	return nil
}

// extractEventInfo pulls repo, sender, and a human-readable summary from the raw payload.
func extractEventInfo(eventType string, payload []byte) (repo, sender, summary string) {
	var raw struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Sender struct {
			Login string `json:"login"`
		} `json:"sender"`
		Ref     string `json:"ref"`
		Commits []any  `json:"commits"`
		Action  string `json:"action"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		slog.Warn("Failed to extract event info from payload", "event_type", eventType, "error", err)
	}

	repo = raw.Repository.FullName
	sender = raw.Sender.Login

	switch eventType {
	case "push":
		branch := raw.Ref
		if strings.HasPrefix(branch, "refs/heads/") {
			branch = strings.TrimPrefix(branch, "refs/heads/")
		}
		n := len(raw.Commits)
		summary = fmt.Sprintf("pushed %d commit(s) to %s", n, branch)
	case "ping":
		summary = "webhook ping"
	default:
		if raw.Action != "" {
			summary = raw.Action
		} else {
			summary = eventType
		}
	}
	return
}

func (s *Service) handlePush(ctx context.Context, event *gh.PushEvent, deliveryID string) {
	repo := event.GetRepo().GetFullName()
	ref := event.GetRef()

	slog.Info("Push event received",
		"delivery_id", deliveryID,
		"repo", repo,
		"ref", ref,
		"pusher", event.GetPusher().GetName(),
		"commits", len(event.Commits),
	)

	if ref != "refs/heads/main" && ref != "refs/heads/master" {
		return
	}

	if event.GetDeleted() {
		slog.Info("Ignoring branch-delete push", "delivery_id", deliveryID, "repo", repo, "ref", ref)
		return
	}

	sha := event.GetAfter()
	if sha == "" || sha == "0000000000000000000000000000000000000000" {
		slog.Info("Ignoring push with zero SHA", "delivery_id", deliveryID, "repo", repo, "ref", ref)
		return
	}

	matches, err := s.mapping.Match(ctx, repo)
	if err != nil {
		slog.Error("Failed to match mappings", "delivery_id", deliveryID, "repo", repo, "error", err)
		return
	}
	if len(matches) == 0 {
		slog.Info("No mapping for repo, skipping", "delivery_id", deliveryID, "repo", repo)
		return
	}

	installationID := event.GetInstallation().GetID()
	if installationID == 0 {
		slog.Warn("No installation ID in push event, skipping deployment creation", "delivery_id", deliveryID, "repo", repo)
		return
	}

	client, err := s.ghClient.GetInstallationClient(ctx, installationID)
	if err != nil {
		slog.Error("Failed to get installation client", "delivery_id", deliveryID, "error", err)
		return
	}

	owner := event.GetRepo().GetOwner().GetLogin()
	repoName := event.GetRepo().GetName()

	commitMsg := ""
	if event.GetHeadCommit() != nil {
		commitMsg = event.GetHeadCommit().GetMessage()
		if idx := strings.IndexByte(commitMsg, '\n'); idx != -1 {
			commitMsg = commitMsg[:idx]
		}
	}

	for i, entry := range matches {
		environment := entry.Environment

		description := fmt.Sprintf("Deployment queued for agent %s", entry.AgentName)
		if commitMsg != "" {
			description = fmt.Sprintf("%s: %s", description, commitMsg)
		}

		deployment, _, err := client.Repositories.CreateDeployment(ctx, owner, repoName, &gh.DeploymentRequest{
			Ref:              &sha,
			Environment:      gh.Ptr(environment),
			Description:      &description,
			AutoMerge:        gh.Ptr(false),
			RequiredContexts: &[]string{},
		})
		if err != nil {
			slog.Error("Failed to create deployment", "delivery_id", deliveryID, "repo", repo, "agent", entry.AgentName, "sha", sha, "error", err)
			continue
		}

		slog.Info("Deployment created", "delivery_id", deliveryID, "repo", repo, "agent", entry.AgentName, "deployment_id", deployment.GetID(), "sha", sha)

		di := &intent.DeployIntent{
			ID:             fmt.Sprintf("%s-%d", deliveryID, i),
			CreatedAt:      time.Now(),
			DeliveryID:     deliveryID,
			StackIndex:     i,
			Repo:           repo,
			Ref:            ref,
			SHA:            sha,
			Stack:          entry.AgentName,
			Services:       entry.Services,
			Environment:    environment,
			DeploymentID:   deployment.GetID(),
			InstallationID: installationID,
			Status:         intent.StatusCreated,
		}
		s.store.Enqueue(di)

		slog.Info("Deploy intent enqueued",
			"delivery_id", deliveryID,
			"intent_id", di.ID,
			"repo", repo,
			"agent", entry.AgentName,
			"services", len(entry.Services),
			"environment", environment,
		)

		_, _, err = client.Repositories.CreateDeploymentStatus(ctx, owner, repoName, deployment.GetID(), &gh.DeploymentStatusRequest{
			State:       gh.Ptr("queued"),
			Description: gh.Ptr(description),
			Environment: gh.Ptr(environment),
		})
		if err != nil {
			slog.Warn("Failed to mark deployment queued; intent still enqueued", "delivery_id", deliveryID, "repo", repo, "agent", entry.AgentName, "deployment_id", deployment.GetID(), "error", err)
		}
	}
}
