package powershell

import (
	"fmt"
	"os/exec"
)

type Powershell struct{}

func NewPowershell() *Powershell {
	return &Powershell{}
}

func (p *Powershell) Run(args string) (string, error) {
	output, err := exec.Command("powershell", "-command", args).CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to run `powershell -command %s`: %s: %s", args, err.Error(), string(output))
	}

	return string(output), nil
}
