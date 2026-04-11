package mapping

import "strings"

// Config holds the repo->stack mapping loaded from application.yml.
type Config struct {
	Mappings []Entry `mapstructure:"mappings"`
}

// Entry declares that pushes to Repo should trigger deploys on Stack.
type Entry struct {
	Repo        string   `mapstructure:"repo"`        // "owner/name" - required
	Stack       string   `mapstructure:"stack"`       // compose stack name - required
	Services    []string `mapstructure:"services"`    // optional; empty = whole stack
	Environment string   `mapstructure:"environment"` // optional; default "production"
	Image       string   `mapstructure:"image"`       // optional; overrides convention
	Tag         string   `mapstructure:"tag"`         // optional; overrides {sha}
}

// ResolveImage returns the explicit override if set, otherwise the default
// convention ghcr.io/<lowered-repo>.
func (e Entry) ResolveImage(repo string) string {
	if e.Image != "" {
		return e.Image
	}
	return "ghcr.io/" + strings.ToLower(repo)
}

// ResolveTag returns the explicit override if set, otherwise the commit SHA.
func (e Entry) ResolveTag(sha string) string {
	if e.Tag != "" {
		return e.Tag
	}
	return sha
}

// ResolveEnvironment returns the explicit environment if set, otherwise "production".
func (e Entry) ResolveEnvironment() string {
	if e.Environment != "" {
		return e.Environment
	}
	return "production"
}

// Mapping is the in-memory index of configured repo->stack entries.
type Mapping struct {
	entries []Entry
}

func New(entries []Entry) *Mapping {
	return &Mapping{entries: entries}
}

// Match returns every entry whose Repo matches (case-insensitive). A push can
// legitimately produce multiple intents if the same repo is mapped to several
// stacks.
func (m *Mapping) Match(repo string) []Entry {
	matches := make([]Entry, 0, len(m.entries))
	for _, entry := range m.entries {
		if strings.EqualFold(entry.Repo, repo) {
			matches = append(matches, entry)
		}
	}
	return matches
}
