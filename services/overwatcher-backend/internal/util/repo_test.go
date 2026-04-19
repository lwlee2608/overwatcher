package util

import "testing"

func TestSplitRepo(t *testing.T) {
	cases := []struct {
		in     string
		owner  string
		name   string
		wantOK bool
	}{
		{"owner/repo", "owner", "repo", true},
		{"a/b/c", "a", "b/c", true},
		{"no-slash", "", "", false},
		{"/only-trailing", "", "", false},
		{"only-leading/", "", "", false},
		{"", "", "", false},
	}
	for _, tc := range cases {
		owner, name, ok := SplitRepo(tc.in)
		if ok != tc.wantOK || owner != tc.owner || name != tc.name {
			t.Errorf("SplitRepo(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.in, owner, name, ok, tc.owner, tc.name, tc.wantOK)
		}
	}
}
