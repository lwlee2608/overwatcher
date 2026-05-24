package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInstallHandler_TemplatesVars(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewInstallHandler("agent-v0.3.0", "https://overwatch.example.com")
	r := gin.New()
	r.GET("/install.sh", h.Serve)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/install.sh", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()

	// Placeholders must be substituted, not echoed.
	for _, marker := range []string{"{{COORDINATOR_URL}}", "{{RELEASE_TAG}}"} {
		if strings.Contains(body, marker) {
			t.Errorf("placeholder %q left in rendered script", marker)
		}
	}
	if !strings.Contains(body, `COORDINATOR_URL="https://overwatch.example.com"`) {
		t.Errorf("expected coordinator URL substituted; got body:\n%s", body)
	}
	if !strings.Contains(body, `RELEASE_TAG="agent-v0.3.0"`) {
		t.Errorf("expected release tag substituted; got body:\n%s", body)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/x-shellscript") {
		t.Errorf("Content-Type = %q, want text/x-shellscript", ct)
	}
}

func TestInstallHandler_FallsBackToHostHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewInstallHandler("latest", "") // no PublicURL
	r := gin.New()
	r.GET("/install.sh", h.Serve)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/install.sh", nil)
	req.Host = "agent-host.example.org"
	r.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `COORDINATOR_URL="http://agent-host.example.org"`) {
		t.Errorf("expected Host header fallback; got body:\n%s", body)
	}
}

func TestInstallHandler_HonorsForwardedProto(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewInstallHandler("latest", "")
	r := gin.New()
	r.GET("/install.sh", h.Serve)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/install.sh", nil)
	req.Host = "agent-host.example.org"
	req.Header.Set("X-Forwarded-Proto", "https")
	r.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `COORDINATOR_URL="https://agent-host.example.org"`) {
		t.Errorf("expected https from X-Forwarded-Proto; got body:\n%s", body)
	}
}

func TestInstallHandler_DefaultsToHttpsBehindProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Proxy is in front (X-Forwarded-For present) but didn't set Proto. The
	// handler must still produce https — otherwise the served install.sh
	// writes a broken http coordinator URL into the agent env file.
	cases := []struct {
		name   string
		header string
		value  string
	}{
		{"X-Forwarded-For", "X-Forwarded-For", "203.0.113.1"},
		{"X-Forwarded-Host", "X-Forwarded-Host", "agent-host.example.org"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewInstallHandler("latest", "")
			r := gin.New()
			r.GET("/install.sh", h.Serve)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/install.sh", nil)
			req.Host = "agent-host.example.org"
			req.Header.Set(tc.header, tc.value)
			r.ServeHTTP(rr, req)

			body := rr.Body.String()
			if !strings.Contains(body, `COORDINATOR_URL="https://agent-host.example.org"`) {
				t.Errorf("expected https default behind proxy; got body:\n%s", body)
			}
		})
	}
}
