package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

func readPassword(input *os.File, prompt io.Writer) ([]byte, error) {
	fd := int(input.Fd())
	state, err := term.GetState(fd)
	if err != nil {
		return nil, err
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-signals:
			// ReadPassword's defer cannot run when a signal terminates the process.
			_ = term.Restore(fd, state)
			fmt.Fprintln(prompt, "\nLogin interrupted.")
			os.Exit(1)
		case <-done:
		}
	}()
	defer func() {
		signal.Stop(signals)
		close(done)
		<-stopped
	}()
	fmt.Fprint(prompt, "API key: ")
	return term.ReadPassword(fd)
}
