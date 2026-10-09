package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestAndRawJSON(t *testing.T) {
	raw := `{"projects":[],"future_field":true}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/projects", r.URL.Path)
		require.Equal(t, "Bearer owk_test", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		fmt.Fprint(w, raw)
	}))
	defer server.Close()
	c, err := New(server.URL+"/", "owk_test")
	require.NoError(t, err)
	got, err := c.ListProjects(context.Background())
	require.NoError(t, err)
	require.Empty(t, got.Data.Projects)
	require.Equal(t, raw, string(got.Raw))
}

func TestErrors(t *testing.T) {
	for _, tc := range []struct {
		status        int
		body, message string
	}{
		{401, `{"error":"invalid api key"}`, "invalid api key"},
		{403, `{"error":"forbidden"}`, "forbidden"},
		{502, `<html>bad gateway</html>`, "Bad Gateway"},
	} {
		t.Run(tc.message, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			c, err := New(server.URL, "test")
			require.NoError(t, err)
			_, err = c.ListProjects(context.Background())
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			require.Equal(t, tc.status, apiErr.StatusCode)
			require.Equal(t, tc.message, apiErr.Message)
		})
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	reached := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	c, err := New(server.URL, "secret")
	require.NoError(t, err)
	_, err = c.Version(context.Background())
	require.Error(t, err)
	require.False(t, reached)
}

func TestValidationAndCancellation(t *testing.T) {
	for _, value := range []string{"", "localhost", "ftp://localhost", "http://user:secret@localhost", "http://localhost?query=1", "http://localhost#fragment"} {
		_, err := New(value, "test")
		require.Error(t, err)
		require.NotContains(t, err.Error(), "secret")
	}
	c, err := New("http://127.0.0.1:1", "test")
	require.NoError(t, err)
	for _, id := range []string{"", "..", "/version", "a/b"} {
		_, err := c.GetProject(context.Background(), id)
		require.EqualError(t, err, "invalid project ID")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.Version(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "not json") }))
	defer server.Close()
	c, err := New(server.URL, "test")
	require.NoError(t, err)
	_, err = c.Version(context.Background())
	require.ErrorContains(t, err, "decode API response")
}
