package agent

import (
	"os"
	"testing"
	"time"
)

func TestInitConfig_FallsBackToEmbeddedYAML(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("AGENT_NAME", "embed-test")
	t.Setenv("AGENT_SHARED_SECRET", "secret")
	t.Setenv("AGENT_COORDINATOR_URL", "https://example.test")
	// AGENT_POLL_TIMEOUT intentionally unset — embedded default must win.

	cfg, err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig with no on-disk yaml: %v", err)
	}

	if cfg.Agent.Name != "embed-test" {
		t.Errorf("Name = %q, want %q", cfg.Agent.Name, "embed-test")
	}
	if cfg.Agent.CoordinatorURL != "https://example.test" {
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
	t.Setenv("AGENT_COORDINATOR_URL", "https://example.test")
	t.Setenv("AGENT_POLL_TIMEOUT", "5s")

	cfg, err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	if want := 5 * time.Second; cfg.Agent.PollTimeout != want {
		t.Errorf("PollTimeout = %v, want %v (env override)", cfg.Agent.PollTimeout, want)
	}
}

func TestValidateCoordinatorURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https remote", "https://overwatcher.example.com", false},
		{"http localhost", "http://localhost:8080", false},
		{"http 127.0.0.1", "http://127.0.0.1:8080", false},
		{"http ipv6 loopback", "http://[::1]:8080", false},
		{"http remote rejected", "http://overwatcher.example.com", true},
		{"http remote with port rejected", "http://overwatcher.example.com:80", true},
		{"unsupported scheme", "ftp://example.com", true},
		{"garbage", "::not a url", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCoordinatorURL(tc.url)
			if tc.wantErr && err == nil {
				t.Fatalf("validateCoordinatorURL(%q) = nil, want error", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateCoordinatorURL(%q) = %v, want nil", tc.url, err)
			}
		})
	}
}
