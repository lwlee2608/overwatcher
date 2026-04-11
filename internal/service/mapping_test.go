package service

import "testing"

func TestMappingEntry_ResolveImage(t *testing.T) {
	tests := []struct {
		name  string
		entry MappingEntry
		repo  string
		want  string
	}{
		{"default convention lowers repo", MappingEntry{}, "Owner/Some-Repo", "ghcr.io/owner/some-repo"},
		{"override wins", MappingEntry{Image: "registry.example.com/app"}, "Owner/Some-Repo", "registry.example.com/app"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.entry.ResolveImage(tc.repo); got != tc.want {
				t.Errorf("ResolveImage(%q) = %q, want %q", tc.repo, got, tc.want)
			}
		})
	}
}

func TestMappingEntry_ResolveTag(t *testing.T) {
	tests := []struct {
		name  string
		entry MappingEntry
		sha   string
		want  string
	}{
		{"default is sha", MappingEntry{}, "abc123", "abc123"},
		{"override wins", MappingEntry{Tag: "latest"}, "abc123", "latest"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.entry.ResolveTag(tc.sha); got != tc.want {
				t.Errorf("ResolveTag(%q) = %q, want %q", tc.sha, got, tc.want)
			}
		})
	}
}

func TestMappingEntry_ResolveEnvironment(t *testing.T) {
	if got := (MappingEntry{}).ResolveEnvironment(); got != "production" {
		t.Errorf("default env = %q, want production", got)
	}
	if got := (MappingEntry{Environment: "staging"}).ResolveEnvironment(); got != "staging" {
		t.Errorf("override env = %q, want staging", got)
	}
}

func TestMapping_Match(t *testing.T) {
	entries := []MappingEntry{
		{Repo: "owner/foo", Stack: "foo-stack"},
		{Repo: "Owner/Bar", Stack: "bar-stack"},
		{Repo: "owner/foo", Stack: "foo-stack-canary"},
	}
	m := NewMapping(entries)

	t.Run("case-insensitive match", func(t *testing.T) {
		got := m.Match("OWNER/BAR")
		if len(got) != 1 || got[0].Stack != "bar-stack" {
			t.Errorf("got %+v, want single bar-stack", got)
		}
	})

	t.Run("multi-stack fan-out", func(t *testing.T) {
		got := m.Match("owner/foo")
		if len(got) != 2 {
			t.Fatalf("got %d matches, want 2", len(got))
		}
		if got[0].Stack != "foo-stack" || got[1].Stack != "foo-stack-canary" {
			t.Errorf("unexpected stacks: %+v", got)
		}
	})

	t.Run("no match", func(t *testing.T) {
		if got := m.Match("owner/missing"); len(got) != 0 {
			t.Errorf("got %+v, want empty", got)
		}
	})

	t.Run("empty mapping", func(t *testing.T) {
		if got := NewMapping(nil).Match("owner/foo"); len(got) != 0 {
			t.Errorf("got %+v, want empty", got)
		}
	})
}
