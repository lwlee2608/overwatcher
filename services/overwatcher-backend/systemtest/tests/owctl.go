package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/client"
	"github.com/stretchr/testify/require"
)

func TestOwctl(t *testing.T, router *gin.Engine, sessionToken string) {
	server := httptest.NewServer(router)
	defer server.Close()
	body, err := json.Marshal(dto.CreateAPIKeyRequest{Name: "owctl-systemtest"})
	require.NoError(t, err)
	rr := doJSON(t, router, "POST", "/api/v1/api-keys", body, sessionToken)
	require.Equal(t, http.StatusCreated, rr.Code)
	var key dto.CreateAPIKeyResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &key))
	defer doJSON(t, router, "DELETE", "/api/v1/api-keys/"+key.ID, nil, sessionToken)
	body, err = json.Marshal(dto.CreateProjectRequest{Name: "owctl-project", ComposeFile: "/tmp/compose.yml", Environment: "test"})
	require.NoError(t, err)
	rr = doJSON(t, router, "POST", "/api/v1/projects", body, sessionToken)
	require.Equal(t, http.StatusCreated, rr.Code)
	var project dto.ProjectResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &project))
	defer doJSON(t, router, "DELETE", "/api/v1/projects/"+project.ID, nil, sessionToken)

	c, err := client.New(server.URL, key.Key)
	require.NoError(t, err)
	list, err := c.ListProjects(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, list.Data.Projects)
	got, err := c.GetProject(context.Background(), project.ID)
	require.NoError(t, err)
	require.Equal(t, project.ID, got.Data.ID)
	version, err := c.Version(context.Background())
	require.NoError(t, err)
	require.Equal(t, "systemtest-version", version.Data.Version)
	_, err = c.GetProject(context.Background(), "00000000-0000-0000-0000-000000000000")
	var apiErr *client.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusNotFound, apiErr.StatusCode)
	require.Contains(t, apiErr.Message, "project not found")

	build := exec.Command("make", "build-owctl")
	build.Dir = ".."
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)
	binary, err := filepath.Abs("../bin/owctl")
	require.NoError(t, err)
	home := t.TempDir()
	login := exec.Command(binary, "login", "--url", server.URL)
	login.Env = append(os.Environ(), "HOME="+home, "OVERWATCHER_URL=http://127.0.0.1:1", "OVERWATCHER_API_KEY=")
	terminal, err := pty.Start(login)
	require.NoError(t, err)
	defer terminal.Close()
	defer login.Process.Kill()
	loginOutput := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(terminal)
		var output strings.Builder
		for {
			b, err := reader.ReadByte()
			if err != nil {
				break
			}
			output.WriteByte(b)
			if strings.HasSuffix(output.String(), "API key: ") {
				// ReadPassword switches echo off immediately after the prompt.
				time.Sleep(100 * time.Millisecond)
				_, _ = terminal.Write([]byte(key.Key + "\n"))
			}
		}
		loginOutput <- output.String()
	}()
	select {
	case output := <-loginOutput:
		require.NoError(t, login.Wait(), output)
		require.Contains(t, output, "API key saved.")
		require.NotContains(t, output, key.Key)
	case <-time.After(10 * time.Second):
		t.Fatal("login timed out")
	}
	info, err := os.Stat(filepath.Join(home, ".config", "owctl", "config.yaml"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	run := func(keyValue string, args ...string) (string, error) {
		cmd := exec.Command(binary, args...)
		cmd.Env = append(os.Environ(), "HOME="+home, "OVERWATCHER_URL="+server.URL, "OVERWATCHER_API_KEY="+keyValue)
		output, err := cmd.CombinedOutput()
		require.NotContains(t, string(output), key.Key)
		return string(output), err
	}
	// Empty environment overrides fall back to the config saved by login.
	fromConfig := exec.Command(binary, "project", "list")
	fromConfig.Env = append(os.Environ(), "HOME="+home, "OVERWATCHER_URL=", "OVERWATCHER_API_KEY=")
	output, err = fromConfig.CombinedOutput()
	require.NoError(t, err, "%s", output)
	require.Contains(t, string(output), project.Name)
	outputText, err := run(key.Key, "project", "list")
	require.NoError(t, err, outputText)
	require.Contains(t, outputText, "ID")
	require.Contains(t, outputText, project.Name)
	outputText, err = run(key.Key, "project", "get", project.ID)
	require.NoError(t, err, outputText)
	require.Contains(t, outputText, project.ID)
	outputText, err = run(key.Key, "project", "get", project.ID, "--json")
	require.NoError(t, err, outputText)
	require.JSONEq(t, string(got.Raw), outputText)
	outputText, err = run(key.Key, "--json", "project", "list")
	require.NoError(t, err, outputText)
	require.JSONEq(t, string(list.Raw), outputText)
	outputText, err = run(key.Key, "version")
	require.NoError(t, err, outputText)
	require.Contains(t, outputText, "Client: ")
	require.Contains(t, outputText, "Server: systemtest-version")
	outputText, err = run(key.Key, "version", "--json")
	require.NoError(t, err, outputText)
	require.JSONEq(t, string(version.Raw), outputText)
	outputText, err = run("owk_invalid", "project", "list")
	require.Error(t, err)
	require.Contains(t, outputText, "invalid api key")
	outputText, err = run(key.Key, "project", "get", "00000000-0000-0000-0000-000000000000")
	require.Error(t, err)
	require.Contains(t, outputText, "project not found")
}
