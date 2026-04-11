package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	internalgithub "github.com/lwlee2608/overwatcher/internal/github"

	gh "github.com/google/go-github/v84/github"
)

type WebhookService struct {
	ghClient    *internalgithub.Client
	mapping     *Mapping
	intentStore *IntentStore
}

func NewWebhookService(ghClient *internalgithub.Client, mapping *Mapping, store *IntentStore) *WebhookService {
	return &WebhookService{ghClient: ghClient, mapping: mapping, intentStore: store}
}

func (s *WebhookService) HandleEvent(ctx context.Context, eventType string, deliveryID string, payload []byte) error {
	event, err := gh.ParseWebHook(eventType, payload)
	if err != nil {
		slog.Error("Failed to parse webhook payload", "event", eventType, "delivery_id", deliveryID, "error", err)
		return err
	}

	switch eventType {
	case internalgithub.EventPush:
		s.handlePush(ctx, event.(*gh.PushEvent), deliveryID)
	case internalgithub.EventDeployment:
		s.handleDeployment(event.(*gh.DeploymentEvent), deliveryID)
	case internalgithub.EventDeploymentStatus:
		s.handleDeploymentStatus(event.(*gh.DeploymentStatusEvent), deliveryID)
	case internalgithub.EventWorkflowRun:
		s.handleWorkflowRun(event.(*gh.WorkflowRunEvent), deliveryID)
	case internalgithub.EventCheckRun:
		s.handleCheckRun(event.(*gh.CheckRunEvent), deliveryID)
	case internalgithub.EventCheckSuite:
		s.handleCheckSuite(event.(*gh.CheckSuiteEvent), deliveryID)
	default:
		slog.Info("Unhandled webhook event", "event", eventType, "delivery_id", deliveryID)
	}

	return nil
}

func (s *WebhookService) handlePush(ctx context.Context, event *gh.PushEvent, deliveryID string) {
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

	matches := s.mapping.Match(repo)
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

	for _, entry := range matches {
		image := entry.ResolveImage(repo)
		tag := entry.ResolveTag(sha)
		environment := entry.ResolveEnvironment()

		description := fmt.Sprintf("Deployment queued for stack %s", entry.Stack)
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
			slog.Error("Failed to create deployment", "delivery_id", deliveryID, "repo", repo, "stack", entry.Stack, "sha", sha, "error", err)
			continue
		}

		slog.Info("Deployment created", "delivery_id", deliveryID, "repo", repo, "stack", entry.Stack, "deployment_id", deployment.GetID(), "sha", sha)

		_, _, err = client.Repositories.CreateDeploymentStatus(ctx, owner, repoName, deployment.GetID(), &gh.DeploymentStatusRequest{
			State:       gh.Ptr("queued"),
			Description: gh.Ptr("Deployment queued"),
			Environment: gh.Ptr(environment),
		})
		if err != nil {
			slog.Error("Failed to create deployment status", "delivery_id", deliveryID, "repo", repo, "stack", entry.Stack, "deployment_id", deployment.GetID(), "error", err)
			continue
		}

		intent := &DeployIntent{
			ID:           fmt.Sprintf("%s-%s", deliveryID, entry.Stack),
			CreatedAt:    time.Now(),
			DeliveryID:   deliveryID,
			Repo:         repo,
			Ref:          ref,
			SHA:          sha,
			Image:        image,
			Tag:          tag,
			Stack:        entry.Stack,
			Services:     entry.Services,
			Environment:  environment,
			DeploymentID: deployment.GetID(),
			Status:       IntentCreated,
		}
		s.intentStore.Enqueue(intent)

		slog.Info("Deploy intent enqueued",
			"delivery_id", deliveryID,
			"intent_id", intent.ID,
			"repo", repo,
			"stack", entry.Stack,
			"image", image,
			"tag", tag,
			"environment", environment,
		)
	}
}

func (s *WebhookService) handleDeployment(event *gh.DeploymentEvent, deliveryID string) {
	slog.Info("Deployment event received",
		"delivery_id", deliveryID,
		"repo", event.GetRepo().GetFullName(),
		"environment", event.GetDeployment().GetEnvironment(),
		"ref", event.GetDeployment().GetRef(),
	)
}

func (s *WebhookService) handleDeploymentStatus(event *gh.DeploymentStatusEvent, deliveryID string) {
	slog.Info("Deployment status event received",
		"delivery_id", deliveryID,
		"repo", event.GetRepo().GetFullName(),
		"environment", event.GetDeployment().GetEnvironment(),
		"state", event.GetDeploymentStatus().GetState(),
	)
}

func (s *WebhookService) handleWorkflowRun(event *gh.WorkflowRunEvent, deliveryID string) {
	slog.Info("Workflow run event received",
		"delivery_id", deliveryID,
		"repo", event.GetRepo().GetFullName(),
		"workflow", event.GetWorkflow().GetName(),
		"action", event.GetAction(),
		"conclusion", event.GetWorkflowRun().GetConclusion(),
	)
}

func (s *WebhookService) handleCheckRun(event *gh.CheckRunEvent, deliveryID string) {
	slog.Info("Check run event received",
		"delivery_id", deliveryID,
		"repo", event.GetRepo().GetFullName(),
		"name", event.GetCheckRun().GetName(),
		"action", event.GetAction(),
		"status", event.GetCheckRun().GetStatus(),
	)
}

func (s *WebhookService) handleCheckSuite(event *gh.CheckSuiteEvent, deliveryID string) {
	slog.Info("Check suite event received",
		"delivery_id", deliveryID,
		"repo", event.GetRepo().GetFullName(),
		"action", event.GetAction(),
		"status", event.GetCheckSuite().GetStatus(),
		"conclusion", event.GetCheckSuite().GetConclusion(),
	)
}
