package netsh

import (
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/winc/hcs"
	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

//go:generate counterfeiter -o fakes/hcs_client.go --fake-name HCSClient . HCSClient
type HCSClient interface {
	OpenContainer(string) (hcs.Container, error)
}

type Runner struct {
	hcsClient  HCSClient
	id         string
	cmdTimeout time.Duration
}

func NewRunner(hcsClient HCSClient, containerId string, cmdTimeoutInSeconds int) *Runner {
	return &Runner{
		hcsClient:  hcsClient,
		id:         containerId,
		cmdTimeout: time.Duration(cmdTimeoutInSeconds) * time.Second,
	}
}

func (nr *Runner) RunContainer(args []string) error {
	commandLine := "netsh " + strings.Join(args, " ")
	logrus.Infof("running '%s' in %s", commandLine, nr.id)

	container, err := nr.hcsClient.OpenContainer(nr.id)
	if err != nil {
		return err
	}
	defer container.Close()

	p, err := container.CreateProcess(&hcsshim.ProcessConfig{
		CommandLine: commandLine,
	})
	if err != nil {
		return hcs.CleanError(err)
	}

	if err := p.WaitTimeout(nr.cmdTimeout); err != nil {
		return err
	}

	exitCode, err := p.ExitCode()
	if err != nil {
		return err
	}
	if exitCode != 0 {
		errRet := fmt.Errorf("running '%s' in %s failed: exit code %d", commandLine, nr.id, exitCode)
		logrus.Error(errRet.Error())
		return errRet
	}

	return nil
}
