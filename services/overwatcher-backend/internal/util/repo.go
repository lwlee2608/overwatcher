package util

import "strings"

// SplitRepo parses a GitHub "owner/name" slug into its parts. Returns
// ok=false if the input is malformed — no slash, leading slash, or trailing
// slash.
func SplitRepo(full string) (owner, name string, ok bool) {
	i := strings.IndexByte(full, '/')
	if i <= 0 || i == len(full)-1 {
		return "", "", false
	}
	return full[:i], full[i+1:], true
}
