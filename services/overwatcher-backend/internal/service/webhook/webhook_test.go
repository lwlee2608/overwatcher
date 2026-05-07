package webhook

import "testing"

// Pure-function tests live here. Redeploy precondition tests moved to
// systemtest/tests/webhook.go because they exercise intent.DBStore.

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
