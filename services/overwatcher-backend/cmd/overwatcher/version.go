package main

import "strings"

var AppVersion = "dev"

// resolveAgentReleaseTag picks the GitHub release the install script installs.
// The coordinator installs the agent built from its own release — they're
// tagged in lockstep, so a v0.4.0 coordinator installs the v0.4.0 agent.
// AppVersion is "v0.4.0" (release build) or "v0.4.0-<sha>" (dev/docker build);
// strip the suffix to recover the tag. A "dev" build with no real version
// falls back to "latest".
func resolveAgentReleaseTag(appVersion string) string {
	if tag, _, _ := strings.Cut(appVersion, "-"); strings.HasPrefix(tag, "v") {
		return tag
	}
	return "latest"
}
