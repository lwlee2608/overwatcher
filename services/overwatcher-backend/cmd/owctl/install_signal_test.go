//go:build linux || darwin

package main

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInstallSignalHelper(t *testing.T) {
	if os.Getenv("OWCTL_INSTALL_SIGNAL_HELPER") != "1" {
		return
	}
	os.Args = []string{"owctl", "agent", "install", "--ssh", "fake-vm"}
	main()
	os.Exit(0)
}

func TestInstallConfirmationExitsOnSignal(t *testing.T) {
	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestInstallSignalHelper$")
			command.Env = append(os.Environ(), "OWCTL_INSTALL_SIGNAL_HELPER=1", "HOME="+t.TempDir())
			stdin, err := command.StdinPipe()
			require.NoError(t, err)
			defer stdin.Close()
			stderr, err := command.StderrPipe()
			require.NoError(t, err)
			require.NoError(t, command.Start())
			defer command.Process.Kill()
			ready := make(chan struct{})
			go func() {
				reader := bufio.NewReader(stderr)
				var text strings.Builder
				for {
					b, err := reader.ReadByte()
					if err != nil {
						return
					}
					text.WriteByte(b)
					if strings.HasSuffix(text.String(), "[y/N]: ") {
						close(ready)
						return
					}
				}
			}()
			select {
			case <-ready:
			case <-time.After(5 * time.Second):
				t.Fatal("confirmation prompt missing")
			}
			require.NoError(t, command.Process.Signal(signal))
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			select {
			case err := <-done:
				require.Error(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("confirmation did not exit on signal")
			}
		})
	}
}
