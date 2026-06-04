package main

import (
	"strings"
)

var AppVersion = "dev"

// resolveAgentReleaseTag decides which GitHub release the install script
// installs. An explicit config value always wins. Otherwise the coordinator
// installs the agent built from its own release — they're tagged in lockstep,
// so a v0.4.0 coordinator installs the v0.4.0 agent without manual pinning.
// AppVersion is "v0.4.0" (release build) or "v0.4.0-<sha>" (dev/docker build);
// strip the suffix to recover the tag. A "dev" build with no real version
// falls back to "latest".
func resolveAgentReleaseTag(configured, appVersion string) string {
	if configured != "" {
		return configured
	}
	if tag, _, _ := strings.Cut(appVersion, "-"); strings.HasPrefix(tag, "v") {
		return tag
	}
	return "latest"
}
