package project

import (
	"errors"
	"testing"
)

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

func TestNormalizeRepo(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"owner/repo", "owner/repo"},
		{"  owner/repo  ", "owner/repo"},
		{"https://github.com/owner/repo", "owner/repo"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"https://github.com/owner/repo/", "owner/repo"},
		{"http://github.com/owner/repo", "owner/repo"},
		{"git@github.com:owner/repo.git", "owner/repo"},
		{"git@github.com:owner/repo", "owner/repo"},
		{"Owner-1/repo.name_2", "Owner-1/repo.name_2"},
	}
	for _, tc := range cases {
		got, err := normalizeRepo(tc.in)
		if err != nil {
			t.Errorf("normalizeRepo(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeRepo(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	bad := []string{
		"",
		"   ",
		"foobar",
		"owner/",
		"/repo",
		"owner/repo/extra",
		"owner repo",
		"owner/re$po",
		"https://gitlab.com/owner/repo",
	}
	for _, in := range bad {
		if _, err := normalizeRepo(in); !errors.Is(err, ErrInvalidRepo) {
			t.Errorf("normalizeRepo(%q) err = %v, want ErrInvalidRepo", in, err)
		}
	}
}
