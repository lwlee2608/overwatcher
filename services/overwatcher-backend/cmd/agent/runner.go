package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
)

// Runner executes a deploy intent by shelling out to `docker compose`.
type Runner struct {
	composeFile string
}

func NewRunner(composeFile string) *Runner {
	return &Runner{composeFile: composeFile}
}

// Run executes `docker compose pull` followed by `up -d` for each service in
// the intent. Each service carries its own image and tag, exported as IMAGE
// and IMAGE_TAG for subprocess interpolation. An empty service name falls
// back to applying the command to every service in the compose file.
func (r *Runner) Run(ctx context.Context, intent *dto.DeployIntentResponse) error {
	if len(intent.Services) == 0 {
		return fmt.Errorf("intent has no services")
	}
	for _, svc := range intent.Services {
		env := append(os.Environ(),
			"IMAGE="+svc.Image,
			"IMAGE_TAG="+svc.Tag,
		)

		pullArgs := []string{"compose", "-f", r.composeFile, "pull"}
		upArgs := []string{"compose", "-f", r.composeFile, "up", "-d"}
		if svc.Name != "" {
			pullArgs = append(pullArgs, svc.Name)
			upArgs = append(upArgs, svc.Name)
		}

		if err := r.runDocker(ctx, env, pullArgs...); err != nil {
			return fmt.Errorf("docker compose pull %s: %w", svc.Name, err)
		}
		if err := r.runDocker(ctx, env, upArgs...); err != nil {
			return fmt.Errorf("docker compose up %s: %w", svc.Name, err)
		}
	}
	return nil
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
