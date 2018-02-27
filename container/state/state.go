package state

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/winc/hcs"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const stateFile = "state.json"

//go:generate counterfeiter -o fakes/hcsclient.go --fake-name HCSClient . HCSClient
type HCSClient interface {
	GetContainerProperties(string) (hcsshim.ContainerProperties, error)
	OpenContainer(string) (hcs.Container, error)
}

type Manager struct {
	hcsClient HCSClient
	id        string
	rootDir   string
}

type ContainerState struct {
	Bundle                string           `json:"bundle"`
	UserProgramPID        int              `json:"user_program_pid"`
	UserProgramStartTime  syscall.Filetime `json:"user_program_start_time"`
	UserProgramExecFailed bool             `json:"user_program_exec_failed"`
}

func NewManager(id, rootDir string) *Manager {
	return &Manager{
		id:      id,
		rootDir: rootDir,
	}
}

func (m *Manager) Get() (*specs.State, error) {
	cp, err := m.hcsClient.GetContainerProperties(m.id)
	if err != nil {
		return nil, err
	}

	contents, err := ioutil.ReadFile(filepath.Join(m.stateDir(), stateFile))
	if err != nil {
		return nil, err
	}

	var cs ContainerState
	if err := json.Unmarshal(contents, &cs); err != nil {
		return nil, err
	}

	var status string
	if cp.Stopped {
		status = "stopped"
	} else {
		status, err = m.userProgramStatus(state)
		if err != nil {
			return nil, err
		}
	}

	pid, err := m.containerPid(m.id)
	if err != nil {
		return nil, err
	}

	return &specs.State{
		Version: specs.Version,
		ID:      m.id,
		Status:  status,
		Bundle:  c.Bundle,
		Pid:     pid,
	}, nil
	return nil, nil
}

func (m *Manager) Initialize(bundlePath string) error {
	if err := os.MkdirAll(m.stateDir(), 0755); err != nil {
		return err
	}

	state := ContainerState{Bundle: bundlePath}
	contents, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(m.stateDir(), stateFile), contents, 0644)
}

func (m *Manager) SetRunning(pid uint32) error {
	return nil
}

func (m *Manager) SetExecFailed() error {
	return nil
}

func (m *Manager) stateDir() string {
	return filepath.Join(m.rootDir, m.id)
}
