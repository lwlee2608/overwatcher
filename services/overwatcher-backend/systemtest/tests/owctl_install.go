package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
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
touch "$FAKE_SSH_MARKER"
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
	for _, mode := range []string{"success", "active", "nodocker", "fail", "timeout", "decline", "bind-response-failure", "member"} {
		t.Run(mode, func(t *testing.T) {
			before, err := c.ListAgents(context.Background())
			require.NoError(t, err)
			tokenFile := filepath.Join(t.TempDir(), "received-token")
			sshMarker := filepath.Join(t.TempDir(), "ssh-called")
			apiURL := serverURL
			if mode == "bind-response-failure" || mode == "member" {
				target, parseErr := url.Parse(serverURL)
				require.NoError(t, parseErr)
				proxy := httputil.NewSingleHostReverseProxy(target)
				proxy.ModifyResponse = func(response *http.Response) error {
					if mode == "member" && response.Request.URL.Path == "/api/v1/projects/"+project.Data.ID {
						var payload map[string]any
						if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
							return err
						}
						response.Body.Close()
						payload["role"] = "member"
						body, err := json.Marshal(payload)
						if err != nil {
							return err
						}
						response.Body = io.NopCloser(strings.NewReader(string(body)))
						response.ContentLength = int64(len(body))
						response.Header.Del("Content-Length")
					}
					if mode == "bind-response-failure" && response.Request.Method == "PUT" && strings.HasSuffix(response.Request.URL.Path, "/project") && response.StatusCode == 200 {
						var agent dto.AgentStatusResponse
						body, err := io.ReadAll(response.Body)
						if err != nil {
							return err
						}
						response.Body.Close()
						if err := json.Unmarshal(body, &agent); err != nil {
							return err
						}
						if agent.ProjectID != "" {
							response.StatusCode = 500
							body = []byte(`{"error":"injected failure after committed binding"}`)
						}
						response.Body = io.NopCloser(strings.NewReader(string(body)))
						response.ContentLength = int64(len(body))
						response.Header.Del("Content-Length")
					}
					return nil
				}
				server := httptest.NewServer(proxy)
				defer server.Close()
				apiURL = server.URL
			}
			args := []string{"agent", "install", "--ssh", "deploy@fake-vm", "--project", project.Data.Name, "--timeout", "3s", "--json"}
			if mode != "decline" {
				args = append(args, "--yes")
			}
			command := exec.Command(binary, args...)
			command.Env = append(os.Environ(), "HOME="+home, "OVERWATCHER_URL="+apiURL, "OVERWATCHER_API_KEY="+key, "PATH="+dir+":"+os.Getenv("PATH"), "FAKE_MODE="+mode, "FAKE_URL="+serverURL, "FAKE_TOKEN_FILE="+tokenFile, "REAL_CURL="+realCurl, "FAKE_SSH_MARKER="+sshMarker)
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
				case "member":
					require.Contains(t, output, "project ownership")
					_, statErr := os.Stat(sshMarker)
					require.True(t, os.IsNotExist(statErr))
				case "bind-response-failure":
					require.Contains(t, output, "injected failure after committed binding")
					require.Contains(t, output, "created agent deleted")
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
