package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
)

// Runner executes a deploy intent by shelling out to `docker compose`. The
// compose file path is carried on each intent (projects.compose_file on the
// coordinator) as a path relative to stacksDir.
type Runner struct {
	stacksDir string
	// pullAttempts and pullBackoff configure bounded retry for `docker compose
	// pull` against transient registry-lag errors (the registry can briefly
	// 404 a tag right after the publishing workflow completes). Zero means
	// "use defaults".
	pullAttempts int
	pullBackoff  time.Duration
}

const (
	defaultPullAttempts = 5
	defaultPullBackoff  = 2 * time.Second
)

func NewRunner(stacksDir string) *Runner { return &Runner{stacksDir: stacksDir} }

// resolveComposePath validates and resolves an intent's compose_file under
// stacksDir. The path must be relative, must not contain a '..' segment, and
// — after cleaning — must remain contained within stacksDir lexically. This
// is the only gate on what the agent's docker invocation reads from disk, so
// errors here must surface verbatim rather than be swallowed.
func resolveComposePath(stacksDir, composeFile string) (string, error) {
	if composeFile == "" {
		return "", fmt.Errorf("compose_file is empty")
	}
	if filepath.IsAbs(composeFile) {
		return "", fmt.Errorf("compose_file %q must be relative to AGENT_STACKS_DIR (%s)", composeFile, stacksDir)
	}
	cleaned := filepath.Clean(composeFile)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("compose_file %q escapes AGENT_STACKS_DIR", composeFile)
	}
	resolved := filepath.Join(stacksDir, cleaned)
	// Defense in depth: even after Clean+Join, refuse anything that wandered
	// outside stacksDir (e.g. a symlinked stacksDir handled oddly by a future
	// caller).
	rel, err := filepath.Rel(stacksDir, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("compose_file %q resolves outside AGENT_STACKS_DIR (%s)", composeFile, stacksDir)
	}
	return resolved, nil
}

// Run executes `docker compose pull` followed by `up -d` for each service in
// the intent. Each service carries its own image and tag, exported as IMAGE
// and IMAGE_TAG for subprocess interpolation. An empty service name falls
// back to applying the command to every service in the compose file.
func (r *Runner) Run(ctx context.Context, intent *dto.DeployIntentResponse) error {
	if len(intent.Services) == 0 {
		return fmt.Errorf("intent has no services")
	}
	composePath, err := resolveComposePath(r.stacksDir, intent.ComposeFile)
	if err != nil {
		return err
	}
	if intent.ComposeProjectName == "" {
		return fmt.Errorf("intent has no compose_project_name")
	}
	for _, svc := range intent.Services {
		env := append(os.Environ(),
			"IMAGE="+svc.Image,
			"IMAGE_TAG="+svc.Tag,
		)

		// --project-name is passed per-command (not via COMPOSE_PROJECT_NAME on
		// the agent's env) so an agent that manages more than one stack still
		// scopes each `docker compose` call to the right project namespace.
		pullArgs := []string{"compose", "--project-name", intent.ComposeProjectName, "-f", composePath, "pull"}
		upArgs := []string{"compose", "--project-name", intent.ComposeProjectName, "-f", composePath, "up", "-d"}
		if svc.Name != "" {
			pullArgs = append(pullArgs, svc.Name)
			upArgs = append(upArgs, svc.Name)
		}

		if err := r.runPullWithRetry(ctx, env, pullArgs); err != nil {
			return fmt.Errorf("docker compose pull %s: %w", svc.Name, err)
		}
		if err := r.runDocker(ctx, env, upArgs...); err != nil {
			return fmt.Errorf("docker compose up %s: %w", svc.Name, err)
		}
	}
	return nil
}

// runPullWithRetry runs `docker compose pull` with bounded retries on
// transient manifest-not-yet-published failures. A successful CI run can
// briefly precede registry availability of the new tag, so we don't want
// the deploy to fail on the first try.
func (r *Runner) runPullWithRetry(ctx context.Context, env []string, args []string) error {
	attempts := r.pullAttempts
	if attempts <= 0 {
		attempts = defaultPullAttempts
	}
	backoff := r.pullBackoff
	if backoff <= 0 {
		backoff = defaultPullBackoff
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		err := r.runDocker(ctx, env, args...)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isTransientPullError(err) {
			return err
		}
		if i == attempts-1 {
			break
		}
		wait := backoff << i
		slog.Warn("docker compose pull transient failure, retrying",
			"attempt", i+1, "max_attempts", attempts, "wait", wait, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return lastErr
}

// isTransientPullError reports whether a pull failure looks like registry lag
// (image/tag not yet published) rather than a permanent failure. Match only
// manifest-specific markers so unrelated "not found" output (e.g. "command
// not found", auth/socket errors) doesn't trigger pointless retries.
func isTransientPullError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "manifest unknown") ||
		strings.Contains(msg, "manifest for ")
}

func (r *Runner) runDocker(ctx context.Context, env []string, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	slog.Debug("docker exec", "args", args, "output_bytes", len(out))
	if err != nil {
		slog.Error("docker exec failed", "args", args, "output", string(out), "error", err)
		return fmt.Errorf("%w: %s", err, string(out))
	}
	slog.Debug("docker exec output", "args", args, "output", string(out))
	return nil
}

