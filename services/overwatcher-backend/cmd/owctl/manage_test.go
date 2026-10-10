package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lwlee2608/overwatcher/internal/client"
	"github.com/stretchr/testify/require"
)

func TestReadServices(t *testing.T) {
	for _, input := range []string{`{"services":[{"name":"web","repo":"owner/repo","image":"web","root_directory":"app","position":2}]}`, "services:\n  - name: web\n    repo: owner/repo\n    image: web\n    root_directory: app\n    position: 2\n"} {
		got, err := readServices(strings.NewReader(input))
		require.NoError(t, err)
		require.Equal(t, "app", got.Services[0].RootDirectory)
		require.Equal(t, 2, got.Services[0].Position)
	}
	for _, input := range []string{"", "null", "{}", "services: null", "services: nope", "services: []\n---\nservices: []", "services: []\ntypo: true", "services: [{rootdirectory: app}]"} {
		_, err := readServices(strings.NewReader(input))
		require.Error(t, err, input)
	}
	got, err := readServices(strings.NewReader("services: []"))
	require.NoError(t, err)
	require.Empty(t, got.Services)
}

func TestReadServicesPreservesDateText(t *testing.T) {
	for _, input := range []string{
		"services: [{tag: 2026-10-09, branch: 2026-10-09}]",
		"services: [{tag: &date 2026-10-09, branch: *date}]",
		"services: [{tag: '2026-10-09', branch: '2026-10-09'}]",
		`{"services":[{"tag":"2026-10-09","branch":"2026-10-09"}]}`,
	} {
		got, err := readServices(strings.NewReader(input))
		require.NoError(t, err)
		require.Equal(t, "2026-10-09", got.Services[0].Tag)
		require.Equal(t, "2026-10-09", got.Services[0].Branch)
	}
}

func TestResolveProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/projects", r.URL.Path)
		_, _ = w.Write([]byte(`{"projects":[{"id":"first","name":"unique"},{"id":"second","name":"duplicate"},{"id":"third","name":"duplicate"}]}`))
	}))
	defer server.Close()
	c, err := client.New(server.URL, "key")
	require.NoError(t, err)
	id, err := resolveProject(context.Background(), c, "unique")
	require.NoError(t, err)
	require.Equal(t, "first", id)
	_, err = resolveProject(context.Background(), c, "duplicate")
	require.ErrorContains(t, err, "ambiguous")
	_, err = resolveProject(context.Background(), c, "missing")
	require.ErrorContains(t, err, "project not found")
	id, err = resolveProject(context.Background(), c, "00000000-0000-0000-0000-000000000000")
	require.NoError(t, err)
	require.Equal(t, "00000000-0000-0000-0000-000000000000", id)
}
