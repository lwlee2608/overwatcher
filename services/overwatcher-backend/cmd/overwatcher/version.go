package main

import "strings"

var AppVersion = "dev"

// resolveReleaseTag picks the GitHub release the install scripts install.
// The coordinator installs the agent and owctl built from its own release —
// they're tagged in lockstep, so a v0.4.0 coordinator installs v0.4.0 binaries.
// AppVersion is "v0.4.0" (release build) or "v0.4.0-<sha>" (dev/docker build);
// strip the suffix to recover the tag. A "dev" build with no real version
// falls back to "latest".
func resolveReleaseTag(appVersion string) string {
	if tag, _, _ := strings.Cut(appVersion, "-"); strings.HasPrefix(tag, "v") {
		return tag
	}
	return "latest"
}
