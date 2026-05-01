package webhook

import (
	"context"
	"errors"
	"testing"

	"github.com/lwlee2608/overwatcher/internal/service/intent"
)

func TestRedeploy_SourceNotFound(t *testing.T) {
	store := intent.NewMemoryStore()
	svc := New(nil, nil, store, nil)

	err := svc.Redeploy(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, intent.ErrNotFound) {
		t.Fatalf("expected intent.ErrNotFound, got %v", err)
	}
}

func TestRedeploy_MissingInstallationID(t *testing.T) {
	store := intent.NewMemoryStore()
	src := &intent.DeployIntent{
		ID:             "src-1",
		DeliveryID:     "orig-delivery",
		Repo:           "owner/repo",
		Ref:            "refs/heads/main",
		SHA:            "abcdef1234567890",
		Stack:          "prod-agent",
		Services:       []intent.ServiceSpec{{Name: "web", Image: "x", Tag: "v1"}},
		Environment:    "production",
		InstallationID: 0,
		Status:         intent.StatusCreated,
	}
	store.Enqueue(src)

	svc := New(nil, nil, store, nil)
	err := svc.Redeploy(context.Background(), "src-1")
	if !errors.Is(err, ErrNoInstallation) {
		t.Fatalf("expected ErrNoInstallation, got %v", err)
	}
}

func TestRedeploy_MalformedRepo(t *testing.T) {
	store := intent.NewMemoryStore()
	src := &intent.DeployIntent{
		ID:             "src-2",
		DeliveryID:     "orig-delivery",
		Repo:           "no-slash-here",
		SHA:            "abcdef",
		Stack:          "prod-agent",
		Environment:    "production",
		InstallationID: 42,
		Status:         intent.StatusCreated,
	}
	store.Enqueue(src)

	svc := New(nil, nil, store, nil)
	err := svc.Redeploy(context.Background(), "src-2")
	if !errors.Is(err, ErrInvalidRepo) {
		t.Fatalf("expected ErrInvalidRepo, got %v", err)
	}
}

func TestWorkflowFilename(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{".github/workflows/build.yml", "build.yml"},
		{"build.yml", "build.yml"},
		{".github/workflows/nested/build-and-publish.yaml", "build-and-publish.yaml"},
	}
	for _, tc := range cases {
		if got := workflowFilename(tc.in); got != tc.want {
			t.Errorf("workflowFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPathMatchesRoot(t *testing.T) {
	cases := []struct {
		name    string
		root    string
		changed []string
		want    bool
	}{
		{"root /, any path", "/", []string{"README.md"}, true},
		{"empty root, any path", "", []string{"a.go"}, true},
		{"exact prefix match", "/services/web", []string{"services/web/index.ts"}, true},
		{"nested match", "services/web", []string{"services/web/deep/nested/file.ts"}, true},
		{"no changed paths (truncated)", "/services/web", nil, true},
		{"different root", "/services/web", []string{"services/api/main.go"}, false},
		{"root prefix but different dir", "/services/web", []string{"services/webfoo/x"}, false},
		{"exact file match", "/services/web/x.go", []string{"services/web/x.go"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathMatchesRoot(tc.root, tc.changed); got != tc.want {
				t.Errorf("root=%q changed=%v: got %v, want %v", tc.root, tc.changed, got, tc.want)
			}
		})
	}
}
