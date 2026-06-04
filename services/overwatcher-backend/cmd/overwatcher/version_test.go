package main

import "testing"

func TestResolveAgentReleaseTag(t *testing.T) {
	cases := []struct {
		name       string
		appVersion string
		want       string
	}{
		{"derives from dev/docker build", "v0.4.0-abc123", "v0.4.0"},
		{"derives from release build", "v0.4.0", "v0.4.0"},
		{"dev build falls back to latest", "dev", "latest"},
		{"empty version falls back to latest", "", "latest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveAgentReleaseTag(tc.appVersion); got != tc.want {
				t.Errorf("resolveAgentReleaseTag(%q) = %q, want %q",
					tc.appVersion, got, tc.want)
			}
		})
	}
}
