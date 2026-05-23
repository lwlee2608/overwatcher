package agent

import (
	"os"
	"testing"
	"time"
)

// TestInitConfig_FallsBackToEmbeddedYAML runs from a temp directory with no
// application-agent.yml on disk. The binary must still come up with defaults
// from the embedded copy (e.g. poll_timeout=30s) and pick up env overrides.
//
// systemd deployments rely on this — there is no on-disk YAML next to the
// installed binary, only /etc/overwatcher-agent.env.
func TestInitConfig_FallsBackToEmbeddedYAML(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("AGENT_NAME", "embed-test")
	t.Setenv("AGENT_SHARED_SECRET", "secret")
	t.Setenv("AGENT_COORDINATOR_URL", "http://example.test")
	// Don't set AGENT_POLL_TIMEOUT — verify the embedded default wins.

	cfg, err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig with no on-disk yaml: %v", err)
	}

	if cfg.Agent.Name != "embed-test" {
		t.Errorf("Name = %q, want %q", cfg.Agent.Name, "embed-test")
	}
	if cfg.Agent.CoordinatorURL != "http://example.test" {
		t.Errorf("CoordinatorURL = %q, want override value", cfg.Agent.CoordinatorURL)
	}
	if want := 30 * time.Second; cfg.Agent.PollTimeout != want {
		t.Errorf("PollTimeout = %v, want %v (from embedded yaml)", cfg.Agent.PollTimeout, want)
	}

	if _, err := os.Stat("application-agent.yml"); err == nil {
		t.Errorf("temp dir unexpectedly has application-agent.yml — invalidates the test")
	}
}

func TestInitConfig_EnvOverridesEmbeddedDefault(t *testing.T) {
	t.Chdir(t.TempDir())

	t.Setenv("AGENT_NAME", "env-override")
	t.Setenv("AGENT_SHARED_SECRET", "secret")
	t.Setenv("AGENT_COORDINATOR_URL", "http://example.test")
	t.Setenv("AGENT_POLL_TIMEOUT", "5s")

	cfg, err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	if want := 5 * time.Second; cfg.Agent.PollTimeout != want {
		t.Errorf("PollTimeout = %v, want %v (env override)", cfg.Agent.PollTimeout, want)
	}
}
