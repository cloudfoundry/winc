package command

import "os/exec"

type Command struct{}

func (*Command) Run(command string, args ...string) error {
	return exec.Command(command, args...).Run()
}

func (*Command) CombinedOutput(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}
