package tests

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/client"
	"github.com/stretchr/testify/require"
)

// Execute the actual remote shell with fake VM commands; sudo receives the
// token on stdin and the fake installer authenticates to the real router.
func testOwctlInstall(t *testing.T, c *client.Client, binary, home, serverURL, key string) {
	dir := t.TempDir()
	scripts := map[string]string{
		"ssh": `#!/bin/sh
for arg do case "$arg" in *owa_*) exit 90;; esac; done
exec sh -s
`,
		"systemctl": `#!/bin/sh
[ "$FAKE_MODE" = active ] && exit 0
exit 3
`,
		"docker": `#!/bin/sh
[ "$FAKE_MODE" != nodocker ]
`,
		"sudo": `#!/bin/sh
[ "$1" = -n ] || exit 91
shift
exec "$@"
`,
		"curl": `#!/bin/sh
while [ "$#" -gt 0 ]; do
 if [ "$1" = -o ]; then shift; output=$1; fi
 shift
done
cat > "$output" <<'INSTALL'
printf '%s' "$AGENT_TOKEN" > "$FAKE_TOKEN_FILE"
echo "$AGENT_TOKEN"
echo "$AGENT_TOKEN" >&2
[ "$FAKE_MODE" != fail ] || exit 42
if [ "$FAKE_MODE" != timeout ]; then
 status=$("$REAL_CURL" -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $AGENT_TOKEN" "$FAKE_URL/api/v1/deploy/next")
 [ "$status" = 412 ]
fi
INSTALL
`,
	}
	for name, script := range scripts {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0700))
	}
	realCurl, err := exec.LookPath("curl")
	require.NoError(t, err)
	project, err := c.CreateProject(context.Background(), dto.CreateProjectRequest{Name: "owctl-install-project", ComposeFile: "/tmp/compose.yml"})
	require.NoError(t, err)
	defer c.DeleteProject(context.Background(), project.Data.ID)
	for _, mode := range []string{"success", "active", "nodocker", "fail", "timeout", "decline"} {
		t.Run(mode, func(t *testing.T) {
			before, err := c.ListAgents(context.Background())
			require.NoError(t, err)
			tokenFile := filepath.Join(t.TempDir(), "received-token")
			args := []string{"agent", "install", "--ssh", "deploy@fake-vm", "--project", project.Data.Name, "--timeout", "3s", "--json"}
			if mode != "decline" {
				args = append(args, "--yes")
			}
			command := exec.Command(binary, args...)
			command.Env = append(os.Environ(), "HOME="+home, "OVERWATCHER_URL="+serverURL, "OVERWATCHER_API_KEY="+key, "PATH="+dir+":"+os.Getenv("PATH"), "FAKE_MODE="+mode, "FAKE_URL="+serverURL, "FAKE_TOKEN_FILE="+tokenFile, "REAL_CURL="+realCurl)
			var stdout, stderr strings.Builder
			command.Stdout = &stdout
			command.Stderr = &stderr
			command.Stdin = strings.NewReader("no\n")
			err = command.Run()
			output := stdout.String() + stderr.String()
			require.NotContains(t, output, key)
			if token, readErr := os.ReadFile(tokenFile); readErr == nil {
				require.NotEmpty(t, token)
				require.NotContains(t, output, string(token))
			}
			after, listErr := c.ListAgents(context.Background())
			require.NoError(t, listErr)
			if mode == "success" {
				require.NoError(t, err, output)
				var agent dto.AgentStatusResponse
				require.NoError(t, json.Unmarshal([]byte(stdout.String()), &agent))
				defer func() {
					_, unbindErr := c.BindAgent(context.Background(), agent.ID, dto.BindAgentProjectRequest{})
					require.NoError(t, unbindErr)
					require.NoError(t, c.DeleteAgent(context.Background(), agent.ID))
				}()
				require.Equal(t, "fake-vm", agent.Name)
				require.Equal(t, project.Data.ID, agent.ProjectID)
				require.NotNil(t, agent.LastSeen)
				require.Len(t, after.Data.Agents, len(before.Data.Agents)+1)
			} else {
				require.Error(t, err, output)
				require.Len(t, after.Data.Agents, len(before.Data.Agents))
				switch mode {
				case "active":
					require.Contains(t, output, "already active")
				case "nodocker":
					require.Contains(t, output, "docker compose")
				case "fail":
					require.Contains(t, output, "SSH install failed")
					require.Contains(t, output, "created agent deleted")
				case "timeout":
					require.Contains(t, output, "deadline exceeded")
					require.Contains(t, output, "created agent deleted")
				case "decline":
					require.Contains(t, output, "cancelled")
				}
			}
		})
	}
}
