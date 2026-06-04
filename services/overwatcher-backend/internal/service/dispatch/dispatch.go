package dispatch

import (
	"context"
	"log/slog"
	"time"

	gh "github.com/google/go-github/v84/github"

	internalgithub "github.com/lwlee2608/overwatcher/internal/github"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
	"github.com/lwlee2608/overwatcher/internal/util"
)

// StatusUpdater is the slice of GitHub API dispatch.Service needs.
type StatusUpdater interface {
	UpdateDeploymentStatus(ctx context.Context, installationID int64, owner, repo string, deploymentID int64, state, description string) error
}

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

type noopStatusUpdater struct{}

func (noopStatusUpdater) UpdateDeploymentStatus(context.Context, int64, string, string, int64, string, string) error {
	return nil
}

type Service struct {
	store   *intent.DBStore
	updater StatusUpdater
}

func New(ghClient *internalgithub.Client, store *intent.DBStore) *Service {
	return &Service{
		store:   store,
		updater: &ghStatusUpdater{client: ghClient},
	}
}

// NewForTest constructs a Service for cross-package tests. A nil updater
// becomes a no-op.
func NewForTest(store *intent.DBStore, updater StatusUpdater) *Service {
	if updater == nil {
		updater = noopStatusUpdater{}
	}
	return &Service{store: store, updater: updater}
}

// Next blocks until an intent for agentName's project is available or ctx is
// cancelled. The in_progress GitHub status update is best-effort; failures
// are logged.
func (d *Service) Next(ctx context.Context, agentName string) (*intent.DeployIntent, error) {
	i, err := d.store.TakeNext(ctx, agentName)
	if err != nil {
		return nil, err
	}

	owner, repoName, _ := util.SplitRepo(i.Repo)
	if err := d.updater.UpdateDeploymentStatus(ctx, i.InstallationID, owner, repoName, i.DeploymentID, "in_progress", "Deploy in progress"); err != nil {
		slog.Warn("Failed to mark deployment in_progress; intent still dispatched",
			"intent_id", i.ID,
			"deployment_id", i.DeploymentID,
			"error", err,
		)
	}

	slog.Info("Intent dispatched to agent",
		"intent_id", i.ID,
		"repo", i.Repo,
		"stack", i.Stack,
		"sha", i.SHA,
	)
	return i, nil
}

// Report records an agent's deploy result and updates the GitHub Deployment
// status. Returns false if the id is unknown or not owned by agentID's project.
func (d *Service) Report(ctx context.Context, id, agentID string, success bool, errMsg string) bool {
	i, ok := d.store.Complete(id, agentID, success)
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

	owner, repoName, _ := util.SplitRepo(i.Repo)
	if err := d.updater.UpdateDeploymentStatus(ctx, i.InstallationID, owner, repoName, i.DeploymentID, state, description); err != nil {
		slog.Warn("Failed to update deployment status",
			"intent_id", id,
			"state", state,
			"error", err,
		)
	}

	slog.Info("Intent completed",
		"intent_id", id,
		"repo", i.Repo,
		"stack", i.Stack,
		"state", state,
	)
	return true
}

func (d *Service) NewReaper(timeout time.Duration, maxAttempts int, interval time.Duration) *Reaper {
	return NewReaper(d.store, d.updater, timeout, maxAttempts, interval)
}

func (d *Service) ListRecent(ctx context.Context, limit int32) ([]*intent.DeployIntent, error) {
	return d.store.ListRecent(ctx, limit)
}

func (d *Service) ListRecentForUser(ctx context.Context, userID string, limit int32) ([]*intent.DeployIntent, error) {
	return d.store.ListRecentForUser(ctx, userID, limit)
}

func (d *Service) ListForUserPaged(ctx context.Context, userID string, filter intent.DeployFilter, limit, offset int32) ([]*intent.DeployIntent, error) {
	return d.store.ListForUserPaged(ctx, userID, filter, limit, offset)
}

func (d *Service) CountForUser(ctx context.Context, userID string, filter intent.DeployFilter) (int64, error) {
	return d.store.CountForUser(ctx, userID, filter)
}

func (d *Service) GetByID(ctx context.Context, id string) (*intent.DeployIntent, error) {
	return d.store.GetByID(ctx, id)
}

