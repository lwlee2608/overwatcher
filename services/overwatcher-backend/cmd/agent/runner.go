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

// Run executes `docker compose pull` followed by `up -d` for the given
// services. The intent's resolved Image and Tag are exposed to the subprocess
// as IMAGE and IMAGE_TAG so compose files can interpolate them
// (e.g. `image: ${IMAGE}:${IMAGE_TAG}`). Returns nil on success or an error
// describing which command failed and its combined output.
func (r *Runner) Run(ctx context.Context, intent *dto.DeployIntentResponse) error {
	env := append(os.Environ(),
		"IMAGE="+intent.Image,
		"IMAGE_TAG="+intent.Tag,
	)

	pullArgs := []string{"compose", "-f", r.composeFile, "pull"}
	pullArgs = append(pullArgs, intent.Services...)
	if err := r.runDocker(ctx, env, pullArgs...); err != nil {
		return fmt.Errorf("docker compose pull: %w", err)
	}

	upArgs := []string{"compose", "-f", r.composeFile, "up", "-d"}
	upArgs = append(upArgs, intent.Services...)
	if err := r.runDocker(ctx, env, upArgs...); err != nil {
		return fmt.Errorf("docker compose up: %w", err)
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
