package cmd

import (
	"bytes"
	"os/exec"
)

// MustConfigure re-configures a command dynamically.
// If the reconfiguration fails it's expected to crash the program
func MustConfigure(cmd *exec.Cmd, fn func(*exec.Cmd)) *exec.Cmd {
	fn(cmd)
	return cmd
}

// Pipeline is a collection of exec.Cmd to execute in serial
type Pipeline []*exec.Cmd

// Run executes all commands in a pipeline and returns the first error it encounters or nil
func (p Pipeline) Run() (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, cmd := range p {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return stdout.String(), stderr.String(), err
		}
	}
	return stdout.String(), stderr.String(), nil
}
