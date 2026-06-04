package main

import "testing"

func TestResolveAgentReleaseTag(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		appVersion string
		want       string
	}{
		{"explicit pin wins", "v0.3.0", "v0.4.0-abc123", "v0.3.0"},
		{"explicit latest wins", "latest", "v0.4.0", "latest"},
		{"derives from dev/docker build", "", "v0.4.0-abc123", "v0.4.0"},
		{"derives from release build", "", "v0.4.0", "v0.4.0"},
		{"dev build falls back to latest", "", "dev", "latest"},
		{"empty version falls back to latest", "", "", "latest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveAgentReleaseTag(tc.configured, tc.appVersion); got != tc.want {
				t.Errorf("resolveAgentReleaseTag(%q, %q) = %q, want %q",
					tc.configured, tc.appVersion, got, tc.want)
			}
		})
	}
}
