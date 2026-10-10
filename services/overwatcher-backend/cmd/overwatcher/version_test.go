package main

import "testing"

func TestResolveReleaseTag(t *testing.T) {
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
			if got := resolveReleaseTag(tc.appVersion); got != tc.want {
				t.Errorf("resolveReleaseTag(%q) = %q, want %q",
					tc.appVersion, got, tc.want)
			}
		})
	}
}
