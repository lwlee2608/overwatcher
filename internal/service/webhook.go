package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	internalgithub "github.com/lwlee2608/overwatcher/internal/github"

	gh "github.com/google/go-github/v84/github"
)

type WebhookService struct {
	ghClient *internalgithub.Client
}

func NewWebhookService(ghClient *internalgithub.Client) *WebhookService {
	return &WebhookService{ghClient: ghClient}
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

	installationID := event.GetInstallation().GetID()
	if installationID == 0 {
		slog.Warn("No installation ID in push event, skipping deployment creation")
		return
	}

	client, err := s.ghClient.GetInstallationClient(ctx, installationID)
	if err != nil {
		slog.Error("Failed to get installation client", "error", err)
		return
	}

	parts := strings.SplitN(repo, "/", 2)
	owner, repoName := parts[0], parts[1]
	sha := event.GetAfter()

	// Get the head commit message for the deployment description
	description := ""
	if event.GetHeadCommit() != nil {
		description = event.GetHeadCommit().GetMessage()
		// Truncate to first line
		if idx := strings.IndexByte(description, '\n'); idx != -1 {
			description = description[:idx]
		}
	}

	deployment, _, err := client.Repositories.CreateDeployment(ctx, owner, repoName, &gh.DeploymentRequest{
		Ref:              &sha,
		Environment:      gh.Ptr("production"),
		Description:      &description,
		AutoMerge:        gh.Ptr(false),
		RequiredContexts: &[]string{},
	})
	if err != nil {
		slog.Error("Failed to create deployment", "repo", repo, "sha", sha, "error", err)
		return
	}

	slog.Info("Deployment created", "repo", repo, "deployment_id", deployment.GetID(), "sha", sha)

	// Mark deployment as success
	_, _, err = client.Repositories.CreateDeploymentStatus(ctx, owner, repoName, deployment.GetID(), &gh.DeploymentStatusRequest{
		State:       gh.Ptr("success"),
		Description: gh.Ptr(fmt.Sprintf("Deployed %s to production", sha[:7])),
		Environment: gh.Ptr("production"),
	})
	if err != nil {
		slog.Error("Failed to create deployment status", "repo", repo, "deployment_id", deployment.GetID(), "error", err)
		return
	}

	slog.Info("Deployment status set to success", "repo", repo, "deployment_id", deployment.GetID())
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
