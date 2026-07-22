//go:build !windows

package main

import (
	"io"
	"os"
	"os/exec"
)

func configureBackendProcess(*exec.Cmd) {}

func attachBackendProcess(*os.Process) (io.Closer, error) { return nil, nil }

func interruptBackend(process *os.Process) error { return process.Signal(os.Interrupt) }
