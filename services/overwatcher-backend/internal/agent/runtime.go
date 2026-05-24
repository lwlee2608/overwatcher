package agent

import "os"

// detectAgentType reports how the agent is running so the coordinator can
// display the deployment surface. `/.dockerenv` is created by the Docker
// runtime inside every container; absence implies a host (systemd) install.
func detectAgentType() string {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	return "systemd"
}
