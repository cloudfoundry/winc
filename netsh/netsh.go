package netsh

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"code.cloudfoundry.org/winc/hcs"
	"github.com/Microsoft/hcsshim"
)

const CMD_TIMEOUT = time.Second * 2

//go:generate counterfeiter . HCSClient
type HCSClient interface {
	OpenContainer(string) (hcs.Container, error)
}

type Runner struct {
	hcsClient HCSClient
	id        string
}

func NewRunner(hcsClient HCSClient, containerId string) *Runner {
	return &Runner{
		hcsClient: hcsClient,
		id:        containerId,
	}
}

func (nr *Runner) RunContainer(args []string) error {
	commandLine := "netsh " + strings.Join(args, " ")

	container, err := nr.hcsClient.OpenContainer(nr.id)
	if err != nil {
		return err
	}
	defer container.Close()

	p, err := container.CreateProcess(&hcsshim.ProcessConfig{
		CommandLine: commandLine,
	})
	if err != nil {
		return err
	}

	if err := p.WaitTimeout(CMD_TIMEOUT); err != nil {
		return err
	}

	exitCode, err := p.ExitCode()
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("failed to exec %s in container %s: exit code %d", commandLine, nr.id, exitCode)
	}

	return nil
}

func (nr *Runner) RunHost(args []string) ([]byte, error) {
	return exec.Command("netsh", args...).Output()
}
