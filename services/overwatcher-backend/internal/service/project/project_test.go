package project

import "testing"

func TestNormalizeWorkflow(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"   ", ""},
		{"build.yml", "build.yml"},
		{"  build.yml  ", "build.yml"},
		{".github/workflows/build.yml", "build.yml"},
		{".github/workflows/nested/build-and-publish.yaml", "build-and-publish.yaml"},
	}
	for _, tc := range cases {
		if got := normalizeWorkflow(tc.in); got != tc.want {
			t.Errorf("normalizeWorkflow(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
