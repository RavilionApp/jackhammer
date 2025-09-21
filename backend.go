package main

import (
	"bytes"
	"errors"
	"os/exec"
)

var NotAvailable = errors.New("the selected backend is not available")

type Backend interface {
	isAvailable() bool
	buildCmd(inputUrl string) (*exec.Cmd, error)
	setupLogOutput(cmd *exec.Cmd, buffer *bytes.Buffer)
}
