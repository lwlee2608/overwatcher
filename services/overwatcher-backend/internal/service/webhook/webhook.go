package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	gh "github.com/google/go-github/v84/github"
	"github.com/google/uuid"

	internalgithub "github.com/lwlee2608/overwatcher/internal/github"
	"github.com/lwlee2608/overwatcher/internal/service/eventlog"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
	"github.com/lwlee2608/overwatcher/internal/service/project"
	"github.com/lwlee2608/overwatcher/internal/util"
)

// Errors returned by Service.Redeploy so callers can branch with errors.Is.
var (
	ErrNoInstallation = errors.New("source intent has no GitHub installation id")
	ErrInvalidRepo    = errors.New("invalid repo format")
)

type Service struct {
	ghClient *internalgithub.Client
	projects *project.Service
	store    intent.Store
	eventLog *eventlog.Service
}

func New(ghClient *internalgithub.Client, p *project.Service, store intent.Store, el *eventlog.Service) *Service {
	return &Service{ghClient: ghClient, projects: p, store: store, eventLog: el}
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
	case internalgithub.EventWorkflowRun:
		s.handleWorkflowRun(ctx, event.(*gh.WorkflowRunEvent), deliveryID)
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
		Ref      string `json:"ref"`
		Commits  []any  `json:"commits"`
		Action   string `json:"action"`
		Workflow struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"workflow"`
		WorkflowRun struct {
			HeadBranch string `json:"head_branch"`
			Conclusion string `json:"conclusion"`
		} `json:"workflow_run"`
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
	case "workflow_run":
		wf := raw.Workflow.Name
		if wf == "" {
			wf = path.Base(raw.Workflow.Path)
		}
		summary = fmt.Sprintf("workflow_run %s (%s) on %s: %s",
			raw.Action, wf, raw.WorkflowRun.HeadBranch, raw.WorkflowRun.Conclusion)
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

	branch := strings.TrimPrefix(ref, "refs/heads/")
	if branch == ref {
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

	matches, err := s.projects.ListEnabledServicesByRepoAndBranch(ctx, repo, branch)
	if err != nil {
		slog.Error("Failed to list services for push", "delivery_id", deliveryID, "repo", repo, "branch", branch, "error", err)
		return
	}
	if len(matches) == 0 {
		slog.Info("No enabled service for repo/branch, skipping", "delivery_id", deliveryID, "repo", repo, "branch", branch)
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

	changedPaths := changedPathsFromPush(event)

	// Filter by root_directory ∩ changed paths, then group surviving services
	// by project_id preserving input order (ORDER BY p.id, s.position).
	type group struct {
		projectID   string
		projectName string
		composeFile string
		environment string
		services    []intent.ServiceSpec
	}
	groups := make(map[string]*group)
	var order []string

	for _, m := range matches {
		// Services with a workflow filter deploy via workflow_run, not push.
		if m.Workflow != "" {
			continue
		}
		if !pathMatchesRoot(m.RootDirectory, changedPaths) {
			continue
		}
		g, ok := groups[m.ProjectID]
		if !ok {
			g = &group{
				projectID:   m.ProjectID,
				projectName: m.ProjectName,
				composeFile: m.ProjectComposeFile,
				environment: m.ProjectEnvironment,
			}
			groups[m.ProjectID] = g
			order = append(order, m.ProjectID)
		}
		g.services = append(g.services, intent.ServiceSpec{
			Name:  m.ServiceName,
			Image: m.Image,
			Tag:   m.Tag,
		})
	}

	if len(order) == 0 {
		slog.Info("Push touched no service roots, skipping", "delivery_id", deliveryID, "repo", repo, "branch", branch)
		return
	}

	commitMsg := ""
	if event.GetHeadCommit() != nil {
		commitMsg = event.GetHeadCommit().GetMessage()
		if idx := strings.IndexByte(commitMsg, '\n'); idx != -1 {
			commitMsg = commitMsg[:idx]
		}
	}

	for _, pid := range order {
		g := groups[pid]
		description := fmt.Sprintf("Deployment queued for project %s", g.projectName)
		if commitMsg != "" {
			description = fmt.Sprintf("%s: %s", description, commitMsg)
		}

		deploymentID, err := s.createGitHubDeployment(ctx, client, owner, repoName, sha, g.environment, description)
		if err != nil {
			slog.Error("Failed to create deployment", "delivery_id", deliveryID, "repo", repo, "project", g.projectName, "sha", sha, "error", err)
			continue
		}

		slog.Info("Deployment created", "delivery_id", deliveryID, "repo", repo, "project", g.projectName, "deployment_id", deploymentID, "sha", sha)

		di := &intent.DeployIntent{
			CreatedAt:      time.Now(),
			DeliveryID:     deliveryID,
			ProjectID:      g.projectID,
			ProjectName:    g.projectName,
			ComposeFile:    g.composeFile,
			Repo:           repo,
			Ref:            ref,
			SHA:            sha,
			Stack:          g.projectName,
			Services:       g.services,
			Environment:    g.environment,
			DeploymentID:   deploymentID,
			InstallationID: installationID,
			Status:         intent.StatusCreated,
		}
		s.store.Enqueue(di)

		slog.Info("Deploy intent enqueued",
			"delivery_id", deliveryID,
			"project", g.projectName,
			"repo", repo,
			"services", len(g.services),
			"environment", g.environment,
		)
	}
}

// handleWorkflowRun deploys when a successful CI workflow finishes, ensuring
// the image we pull was actually built by that run. Services without a
// configured workflow are unaffected — they continue to deploy via push.
func (s *Service) handleWorkflowRun(ctx context.Context, event *gh.WorkflowRunEvent, deliveryID string) {
	repo := event.GetRepo().GetFullName()
	action := event.GetAction()
	run := event.GetWorkflowRun()
	wf := event.GetWorkflow()

	slog.Info("workflow_run event received",
		"delivery_id", deliveryID,
		"repo", repo,
		"action", action,
		"conclusion", run.GetConclusion(),
		"workflow", wf.GetPath(),
		"head_branch", run.GetHeadBranch(),
		"head_sha", run.GetHeadSHA(),
	)

	if action != "completed" {
		return
	}
	if run.GetConclusion() != "success" {
		slog.Info("workflow_run not successful, skipping",
			"delivery_id", deliveryID, "repo", repo, "conclusion", run.GetConclusion())
		return
	}

	branch := run.GetHeadBranch()
	sha := run.GetHeadSHA()
	if branch == "" || sha == "" {
		slog.Info("workflow_run missing branch or SHA, skipping", "delivery_id", deliveryID, "repo", repo)
		return
	}

	workflowFile := workflowFilename(wf.GetPath())
	if workflowFile == "" {
		slog.Info("workflow_run missing workflow path, skipping", "delivery_id", deliveryID, "repo", repo)
		return
	}

	matches, err := s.projects.ListEnabledServicesByRepoAndWorkflow(ctx, repo, branch, workflowFile)
	if err != nil {
		slog.Error("Failed to list services for workflow_run",
			"delivery_id", deliveryID, "repo", repo, "branch", branch, "workflow", workflowFile, "error", err)
		return
	}
	if len(matches) == 0 {
		slog.Info("No enabled service for repo/branch/workflow, skipping",
			"delivery_id", deliveryID, "repo", repo, "branch", branch, "workflow", workflowFile)
		return
	}

	installationID := event.GetInstallation().GetID()
	if installationID == 0 {
		slog.Warn("No installation ID in workflow_run event, skipping",
			"delivery_id", deliveryID, "repo", repo)
		return
	}

	client, err := s.ghClient.GetInstallationClient(ctx, installationID)
	if err != nil {
		slog.Error("Failed to get installation client", "delivery_id", deliveryID, "error", err)
		return
	}

	owner := event.GetRepo().GetOwner().GetLogin()
	repoName := event.GetRepo().GetName()

	// Group matching services by project, preserving order — same shape as
	// handlePush, but no path filter (the workflow itself decides what to build).
	type group struct {
		projectID   string
		projectName string
		composeFile string
		environment string
		services    []intent.ServiceSpec
	}
	groups := make(map[string]*group)
	var order []string

	for _, m := range matches {
		g, ok := groups[m.ProjectID]
		if !ok {
			g = &group{
				projectID:   m.ProjectID,
				projectName: m.ProjectName,
				composeFile: m.ProjectComposeFile,
				environment: m.ProjectEnvironment,
			}
			groups[m.ProjectID] = g
			order = append(order, m.ProjectID)
		}
		g.services = append(g.services, intent.ServiceSpec{
			Name:  m.ServiceName,
			Image: m.Image,
			Tag:   m.Tag,
		})
	}

	commitMsg := ""
	if hc := run.GetHeadCommit(); hc != nil {
		commitMsg = hc.GetMessage()
		if idx := strings.IndexByte(commitMsg, '\n'); idx != -1 {
			commitMsg = commitMsg[:idx]
		}
	}

	ref := "refs/heads/" + branch
	for _, pid := range order {
		g := groups[pid]
		description := fmt.Sprintf("Deployment queued for project %s", g.projectName)
		if commitMsg != "" {
			description = fmt.Sprintf("%s: %s", description, commitMsg)
		}

		deploymentID, err := s.createGitHubDeployment(ctx, client, owner, repoName, sha, g.environment, description)
		if err != nil {
			slog.Error("Failed to create deployment",
				"delivery_id", deliveryID, "repo", repo, "project", g.projectName, "sha", sha, "error", err)
			continue
		}

		slog.Info("Deployment created from workflow_run",
			"delivery_id", deliveryID, "repo", repo, "project", g.projectName,
			"deployment_id", deploymentID, "sha", sha, "workflow", workflowFile)

		di := &intent.DeployIntent{
			CreatedAt:      time.Now(),
			DeliveryID:     deliveryID,
			ProjectID:      g.projectID,
			ProjectName:    g.projectName,
			ComposeFile:    g.composeFile,
			Repo:           repo,
			Ref:            ref,
			SHA:            sha,
			Stack:          g.projectName,
			Services:       g.services,
			Environment:    g.environment,
			DeploymentID:   deploymentID,
			InstallationID: installationID,
			Status:         intent.StatusCreated,
		}
		s.store.Enqueue(di)

		slog.Info("Deploy intent enqueued",
			"delivery_id", deliveryID,
			"project", g.projectName,
			"repo", repo,
			"services", len(g.services),
			"environment", g.environment,
			"trigger", "workflow_run",
		)
	}
}

// workflowFilename returns the basename of a workflow path (e.g.
// ".github/workflows/build.yml" -> "build.yml"). Users configure services
// with the filename, which is more stable than the workflow's display name.
func workflowFilename(p string) string {
	if p == "" {
		return ""
	}
	return path.Base(p)
}

// changedPathsFromPush returns the union of modified/added/removed paths across
// every commit in the push payload. GitHub truncates the commits array at 20
// entries, and for very large pushes the file lists can be omitted. In those
// cases an empty slice is returned and pathMatchesRoot will conservatively
// dispatch everything (treat as "unknown, dispatch to be safe").
//
// TODO: for truncated pushes, fetch full commit diff via the Compare API.
func changedPathsFromPush(event *gh.PushEvent) []string {
	seen := map[string]struct{}{}
	for _, c := range event.Commits {
		for _, p := range c.Modified {
			seen[p] = struct{}{}
		}
		for _, p := range c.Added {
			seen[p] = struct{}{}
		}
		for _, p := range c.Removed {
			seen[p] = struct{}{}
		}
	}
	if head := event.GetHeadCommit(); head != nil {
		for _, p := range head.Modified {
			seen[p] = struct{}{}
		}
		for _, p := range head.Added {
			seen[p] = struct{}{}
		}
		for _, p := range head.Removed {
			seen[p] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	return out
}

// pathMatchesRoot reports whether any changed path falls under the service's
// root_directory. Root "/" (or "") matches everything. If no changed paths
// were reported (truncated push), return true so we don't silently drop.
func pathMatchesRoot(root string, changed []string) bool {
	if len(changed) == 0 {
		return true
	}
	norm := strings.Trim(path.Clean("/"+root), "/")
	if norm == "" || norm == "." {
		return true
	}
	for _, p := range changed {
		cp := strings.TrimLeft(path.Clean(p), "/")
		if cp == norm || strings.HasPrefix(cp, norm+"/") {
			return true
		}
	}
	return false
}

// createGitHubDeployment creates a GitHub Deployment and marks it queued.
// The queued status is best-effort — failure is logged but the deployment ID
// is still returned so the caller can enqueue an intent.
func (s *Service) createGitHubDeployment(
	ctx context.Context,
	client *gh.Client,
	owner, repoName, sha, environment, description string,
) (int64, error) {
	deployment, _, err := client.Repositories.CreateDeployment(ctx, owner, repoName, &gh.DeploymentRequest{
		Ref:              &sha,
		Environment:      gh.Ptr(environment),
		Description:      &description,
		AutoMerge:        gh.Ptr(false),
		RequiredContexts: &[]string{},
	})
	if err != nil {
		return 0, err
	}

	_, _, err = client.Repositories.CreateDeploymentStatus(ctx, owner, repoName, deployment.GetID(), &gh.DeploymentStatusRequest{
		State:       gh.Ptr("queued"),
		Description: gh.Ptr(description),
		Environment: gh.Ptr(environment),
	})
	if err != nil {
		slog.Warn("Failed to mark deployment queued; proceeding anyway",
			"repo", owner+"/"+repoName,
			"deployment_id", deployment.GetID(),
			"error", err,
		)
	}
	return deployment.GetID(), nil
}

// Redeploy clones an existing DeployIntent into a new one targeting the same
// project/SHA/services. A fresh GitHub Deployment is created so the manual
// trigger has its own status timeline. The new intent flows through the same
// agent long-poll + concurrency guard as webhook-produced ones.
func (s *Service) Redeploy(ctx context.Context, sourceID string) error {
	src, err := s.store.GetByID(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("load source intent: %w", err)
	}
	if src.InstallationID == 0 {
		return fmt.Errorf("redeploy %s: %w", sourceID, ErrNoInstallation)
	}

	owner, repoName, ok := util.SplitRepo(src.Repo)
	if !ok {
		return fmt.Errorf("redeploy %s: %w: %q", sourceID, ErrInvalidRepo, src.Repo)
	}

	client, err := s.ghClient.GetInstallationClient(ctx, src.InstallationID)
	if err != nil {
		return fmt.Errorf("redeploy %s: get installation client: %w", sourceID, err)
	}

	shortSHA := src.SHA
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	description := fmt.Sprintf("Manual redeploy of %s @ %s", src.Stack, shortSHA)

	deploymentID, err := s.createGitHubDeployment(ctx, client, owner, repoName, src.SHA, src.Environment, description)
	if err != nil {
		return fmt.Errorf("redeploy %s: create deployment: %w", sourceID, err)
	}

	deliveryID := "manual-" + uuid.NewString()
	di := &intent.DeployIntent{
		CreatedAt:      time.Now(),
		DeliveryID:     deliveryID,
		ProjectID:      src.ProjectID,
		ProjectName:    src.ProjectName,
		ComposeFile:    src.ComposeFile,
		Repo:           src.Repo,
		Ref:            src.Ref,
		SHA:            src.SHA,
		Stack:          src.Stack,
		Services:       src.Services,
		Environment:    src.Environment,
		DeploymentID:   deploymentID,
		InstallationID: src.InstallationID,
		Status:         intent.StatusCreated,
	}
	s.store.Enqueue(di)

	slog.Info("Manual redeploy enqueued",
		"source_id", sourceID,
		"delivery_id", deliveryID,
		"repo", src.Repo,
		"stack", src.Stack,
		"sha", src.SHA,
		"deployment_id", deploymentID,
	)
	return nil
}
