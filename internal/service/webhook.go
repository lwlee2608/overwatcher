package service

import (
	"context"
	"log/slog"

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
		s.handlePush(event.(*gh.PushEvent), deliveryID)
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

func (s *WebhookService) handlePush(event *gh.PushEvent, deliveryID string) {
	slog.Info("Push event received",
		"delivery_id", deliveryID,
		"repo", event.GetRepo().GetFullName(),
		"ref", event.GetRef(),
		"pusher", event.GetPusher().GetName(),
		"commits", len(event.Commits),
	)
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
