package container

import (
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func (m *Manager) Exec(processSpec *specs.Process, createIOPipes bool) (hcsshim.Process, error) {
	container, err := m.hcsClient.OpenContainer(m.id)
	if err != nil {
		return nil, err
	}

	env := map[string]string{}
	for _, e := range processSpec.Env {
		v := strings.Split(e, "=")
		env[v[0]] = strings.Join(v[1:], "=")
	}

	pc := &hcsshim.ProcessConfig{
		CommandLine:      makeCmdLine(processSpec.Args),
		CreateStdInPipe:  createIOPipes,
		CreateStdOutPipe: createIOPipes,
		CreateStdErrPipe: createIOPipes,
		WorkingDirectory: processSpec.Cwd,
		User:             processSpec.User.Username,
		Environment:      env,
	}
	p, err := container.CreateProcess(pc)
	if err != nil {
		command := ""
		if len(processSpec.Args) != 0 {
			command = processSpec.Args[0]
		}
		return nil, &CouldNotCreateProcessError{Id: m.id, Command: command}
	}

	return p, nil
}

func makeCmdLine(args []string) string {
	if len(args) > 0 {
		args[0] = filepath.Clean(args[0])
		base := filepath.Base(args[0])
		match, _ := regexp.MatchString(`\.[a-zA-Z]{3}$`, base)
		if !match {
			args[0] += ".exe"
		}
	}
	var s string
	for _, v := range args {
		if s != "" {
			s += " "
		}
		s += syscall.EscapeArg(v)
	}

	return s
}
