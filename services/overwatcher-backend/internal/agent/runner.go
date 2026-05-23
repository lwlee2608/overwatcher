package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lwlee2608/overwatcher/internal/protocol"
)

// Runner executes a deploy intent by shelling out to `docker compose`. The
// compose file path is carried on each intent (projects.compose_file on the
// coordinator) rather than configured per-agent.
type Runner struct {
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

func NewRunner() *Runner { return &Runner{} }

// Run executes `docker compose pull` followed by `up -d` for each service in
// the intent. Each service carries its own image and tag, exported as IMAGE
// and IMAGE_TAG for subprocess interpolation. An empty service name falls
// back to applying the command to every service in the compose file.
func (r *Runner) Run(ctx context.Context, intent *protocol.DeployIntentResponse) error {
	if len(intent.Services) == 0 {
		return fmt.Errorf("intent has no services")
	}
	if intent.ComposeFile == "" {
		return fmt.Errorf("intent has no compose_file")
	}
	for _, svc := range intent.Services {
		env := append(os.Environ(),
			"IMAGE="+svc.Image,
			"IMAGE_TAG="+svc.Tag,
		)

		pullArgs := []string{"compose", "-f", intent.ComposeFile, "pull"}
		upArgs := []string{"compose", "-f", intent.ComposeFile, "up", "-d"}
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

