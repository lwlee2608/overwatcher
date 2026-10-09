package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestSSHTarget(t *testing.T) {
	for _, target := range []string{"vm", "deploy@vm.example", "my-alias", "user@[::1]", "::1"} {
		if !sshTargetPattern.MatchString(target) {
			t.Errorf("rejected %q", target)
		}
	}
	for _, target := range []string{"", "-oProxyCommand=evil", "user@-host", "vm;touch /tmp/evil", "vm\nwhoami", "$(whoami)", "host name"} {
		if sshTargetPattern.MatchString(target) {
			t.Errorf("accepted %q", target)
		}
	}
}

func TestShellQuote(t *testing.T) {
	value := "token'\n$(exit 42); `exit 42`"
	cmd := exec.Command("sh", "-s")
	cmd.Stdin = strings.NewReader("printf '%s' " + shellQuote(value))
	output, err := cmd.Output()
	if err != nil || string(output) != value {
		t.Fatalf("quote round trip failed: %v", err)
	}
}
