package cmd

import "os/exec"

// MustConfigure re-configures a command dynamically.
// If the reconfiguration fails it's expected to crash the program
func MustConfigure(cmd *exec.Cmd, fn func(*exec.Cmd)) *exec.Cmd {
	fn(cmd)
	return cmd
}

// Pipeline is a collection of exec.Cmd to execute in serial
type Pipeline []*exec.Cmd

// Run executes all commands in a pipeline and returns the first error it encounters or nil
func (p Pipeline) Run() error {
	for _, cmd := range p {
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}
