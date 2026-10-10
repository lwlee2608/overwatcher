//go:build linux || darwin

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
	"golang.org/x/term"
)

func TestLoginSignalHelper(t *testing.T) {
	if os.Getenv("OWCTL_LOGIN_SIGNAL_HELPER") != "1" {
		return
	}
	os.Args = []string{"owctl", "login"}
	main()
	os.Exit(0)
}

func TestLoginRestoresTerminalOnSignal(t *testing.T) {
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			home := t.TempDir()
			master, slave, err := pty.Open()
			require.NoError(t, err)
			defer master.Close()
			defer slave.Close()
			original, err := term.GetState(int(slave.Fd()))
			require.NoError(t, err)
			cmd := exec.Command(os.Args[0], "-test.run=^TestLoginSignalHelper$")
			cmd.Env = append(os.Environ(), "OWCTL_LOGIN_SIGNAL_HELPER=1", "HOME="+home, "OVERWATCHER_URL=http://localhost", "OVERWATCHER_API_KEY=")
			cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
			require.NoError(t, cmd.Start())
			defer cmd.Process.Kill()
			// Wait for ReadPassword to change terminal settings, not just print its prompt.
			require.Eventually(t, func() bool {
				state, err := term.GetState(int(slave.Fd()))
				return err == nil && !reflect.DeepEqual(original, state)
			}, 5*time.Second, 10*time.Millisecond)
			require.NoError(t, cmd.Process.Signal(sig))
			exited := make(chan error, 1)
			go func() { exited <- cmd.Wait() }()
			select {
			case err := <-exited:
				require.Error(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("interrupted login did not exit")
			}
			restored, err := term.GetState(int(slave.Fd()))
			require.NoError(t, err)
			require.Equal(t, original, restored)
			_, err = os.Stat(filepath.Join(home, ".config", "owctl", "config.yaml"))
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}
