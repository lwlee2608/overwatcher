package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
)

// stubDocker writes a fake `docker` script onto PATH that records every
// invocation's argv plus the IMAGE/IMAGE_TAG env vars into a log file. Each
// invocation writes one line: `<args>|IMAGE=<v>|IMAGE_TAG=<v>`.
func stubDocker(t *testing.T, exitCode int) (logPath string, restore func()) {
	t.Helper()
	dir := t.TempDir()
	logPath = filepath.Join(dir, "calls.log")

	script := "#!/bin/sh\n" +
		"echo \"$@|IMAGE=${IMAGE}|IMAGE_TAG=${IMAGE_TAG}\" >> " + logPath + "\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	dockerPath := filepath.Join(dir, "docker")
	if err := os.WriteFile(dockerPath, []byte(script), 0755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+":"+oldPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	return logPath, func() { _ = os.Setenv("PATH", oldPath) }
}

func TestRunner_Run_SingleService(t *testing.T) {
	composeFile := filepath.Join(t.TempDir(), "compose.yml")
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	logPath, restore := stubDocker(t, 0)
	defer restore()

	r := NewRunner()
	intent := &dto.DeployIntentResponse{
		ComposeFile: composeFile,
		Services: []dto.ServiceSpecDTO{
			{Name: "app", Image: "ghcr.io/owner/repo", Tag: "abc1234"},
		},
	}

	if err := r.Run(context.Background(), intent); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(out))
	wantLines := []string{
		"compose -f " + composeFile + " pull app|IMAGE=ghcr.io/owner/repo|IMAGE_TAG=abc1234",
		"compose -f " + composeFile + " up -d app|IMAGE=ghcr.io/owner/repo|IMAGE_TAG=abc1234",
	}
	if got != strings.Join(wantLines, "\n") {
		t.Errorf("docker invocations:\n got:  %q\n want: %q", got, strings.Join(wantLines, "\n"))
	}
}

func TestRunner_Run_MultipleServicesUseOwnImageTag(t *testing.T) {
	composeFile := filepath.Join(t.TempDir(), "compose.yml")
	_ = os.WriteFile(composeFile, []byte("services: {}\n"), 0644)

	logPath, restore := stubDocker(t, 0)
	defer restore()

	r := NewRunner()
	intent := &dto.DeployIntentResponse{
		ComposeFile: composeFile,
		Services: []dto.ServiceSpecDTO{
			{Name: "web", Image: "ghcr.io/owner/web", Tag: "v1"},
			{Name: "api", Image: "ghcr.io/owner/api", Tag: "v2"},
		},
	}

	if err := r.Run(context.Background(), intent); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, _ := os.ReadFile(logPath)
	got := strings.TrimSpace(string(out))
	wantLines := []string{
		"compose -f " + composeFile + " pull web|IMAGE=ghcr.io/owner/web|IMAGE_TAG=v1",
		"compose -f " + composeFile + " up -d web|IMAGE=ghcr.io/owner/web|IMAGE_TAG=v1",
		"compose -f " + composeFile + " pull api|IMAGE=ghcr.io/owner/api|IMAGE_TAG=v2",
		"compose -f " + composeFile + " up -d api|IMAGE=ghcr.io/owner/api|IMAGE_TAG=v2",
	}
	if got != strings.Join(wantLines, "\n") {
		t.Errorf("docker invocations:\n got:  %q\n want: %q", got, strings.Join(wantLines, "\n"))
	}
}

func TestRunner_Run_EmptyNameAppliesToWholeStack(t *testing.T) {
	composeFile := filepath.Join(t.TempDir(), "compose.yml")
	_ = os.WriteFile(composeFile, []byte("services: {}\n"), 0644)

	logPath, restore := stubDocker(t, 0)
	defer restore()

	r := NewRunner()
	intent := &dto.DeployIntentResponse{
		ComposeFile: composeFile,
		Services: []dto.ServiceSpecDTO{
			{Name: "", Image: "ghcr.io/owner/repo", Tag: "v1"},
		},
	}
	if err := r.Run(context.Background(), intent); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, _ := os.ReadFile(logPath)
	// `up -d` followed by `|` (the env separator) — no service args between.
	if !strings.Contains(string(out), "up -d|") {
		t.Errorf("expected `up -d` with no service args, got: %q", string(out))
	}
}

func TestRunner_Run_PullFailure(t *testing.T) {
	composeFile := filepath.Join(t.TempDir(), "compose.yml")
	_ = os.WriteFile(composeFile, []byte("x"), 0644)

	_, restore := stubDocker(t, 1)
	defer restore()

	r := NewRunner()
	intent := &dto.DeployIntentResponse{
		ComposeFile: composeFile,
		Services: []dto.ServiceSpecDTO{
			{Name: "app", Image: "ghcr.io/owner/repo", Tag: "v1"},
		},
	}
	err := r.Run(context.Background(), intent)
	if err == nil {
		t.Fatal("expected error from failing pull")
	}
	if !strings.Contains(err.Error(), "docker compose pull") {
		t.Errorf("err did not mention pull: %v", err)
	}
}

func TestRunner_Run_NoServicesReturnsError(t *testing.T) {
	r := NewRunner()
	err := r.Run(context.Background(), &dto.DeployIntentResponse{ComposeFile: filepath.Join(t.TempDir(), "compose.yml")})
	if err == nil {
		t.Fatal("expected error for empty services")
	}
}

func TestRunner_Run_NoComposeFileReturnsError(t *testing.T) {
	r := NewRunner()
	err := r.Run(context.Background(), &dto.DeployIntentResponse{
		Services: []dto.ServiceSpecDTO{{Name: "app", Image: "x", Tag: "v1"}},
	})
	if err == nil {
		t.Fatal("expected error for missing compose file")
	}
}
