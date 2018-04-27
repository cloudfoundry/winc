package state

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/winc/hcs"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const stateFile = "state.json"
const STILL_ACTIVE_EXIT_CODE = uint32(259)

type Manager struct {
	logger      *logrus.Entry
	hcsClient   HCSClient
	sc          WinSyscall
	containerId string
	rootDir     string
}

type State struct {
	Bundle     string           `json:"bundle"`
	PID        int              `json:"pid"`
	StartTime  syscall.Filetime `json:"start_time"`
	ExecFailed bool             `json:"exec_failed"`
}

//go:generate counterfeiter -o fakes/hcsclient.go --fake-name HCSClient . HCSClient
type HCSClient interface {
	GetContainerProperties(string) (hcsshim.ContainerProperties, error)
}

//go:generate counterfeiter -o fakes/winsyscall.go --fake-name WinSyscall . WinSyscall
type WinSyscall interface {
	OpenProcess(uint32, bool, uint32) (syscall.Handle, error)
	GetProcessStartTime(syscall.Handle) (syscall.Filetime, error)
	CloseHandle(syscall.Handle) error
	GetExitCodeProcess(syscall.Handle) (uint32, error)
}

func New(logger *logrus.Entry, hcsClient HCSClient, winSyscall WinSyscall, id, rootDir string) *Manager {
	return &Manager{
		logger:      logger,
		hcsClient:   hcsClient,
		sc:          winSyscall,
		containerId: id,
		rootDir:     rootDir,
	}
}

func (m *Manager) Initialize(bundlePath string) error {
	if err := os.MkdirAll(m.stateDir(), 0755); err != nil {
		return err
	}

	state := State{Bundle: bundlePath}
	return m.writeState(state)
}

func (m *Manager) Delete() error {
	return os.RemoveAll(m.stateDir())
}

func (m *Manager) SetFailure() error {
	state, err := m.loadState()
	if err != nil {
		return err
	}

	state.PID = 0
	state.StartTime = syscall.Filetime{}
	state.ExecFailed = true
	return m.writeState(state)
}

func (m *Manager) SetSuccess(proc hcs.Process) error {
	state, err := m.loadState()
	if err != nil {
		return err
	}

	// trying to open the process to get a handle + its start time should be valid
	// here, since the hcsshim.process struct has an open handle to the process,
	// and the PID will not be reused until all open handles are closed.
	//
	// https://blogs.msdn.microsoft.com/oldnewthing/20110107-00/?p=11803

	state.PID = proc.Pid()
	h, err := m.sc.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(state.PID))
	if err != nil {
		retErr := fmt.Errorf("OpenProcess: %s", err.Error())
		m.logger.Error(retErr)
		state.ExecFailed = true
		m.writeState(state)
		return retErr
	}
	defer m.sc.CloseHandle(h)

	creationTime, err := m.sc.GetProcessStartTime(h)
	if err != nil {
		retErr := fmt.Errorf("GetProcessStartTime: %s", err.Error())
		m.logger.Error(retErr)
		state.ExecFailed = true
		m.writeState(state)
		return retErr
	}

	state.StartTime = creationTime
	return m.writeState(state)
}

func (m *Manager) State() (*specs.State, error) {
	cp, err := m.hcsClient.GetContainerProperties(m.containerId)
	if err != nil {
		return nil, err
	}

	state, err := m.loadState()
	if err != nil {
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

	return &specs.State{
		Version: specs.Version,
		ID:      m.containerId,
		Status:  status,
		Bundle:  state.Bundle,
		Pid:     state.PID,
	}, nil
}

func (m *Manager) userProgramStatus(state State) (string, error) {
	if state.ExecFailed {
		return "stopped", nil
	}

	if !stateValid(state) {
		return "", fmt.Errorf("invalid state: PID %d, start time %+v", state.PID, state.StartTime)
	}

	if (state.PID == 0) && (state.StartTime == syscall.Filetime{}) {
		return "created", nil
	}

	h, err := m.sc.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(state.PID))
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok {
			// 0x57 is ERROR_INVALID_PARAMETER, which is returned if the process doesn't exist
			if errno == 0x57 {
				return "stopped", nil
			}
		}
		return "", fmt.Errorf("OpenProcess: %s", err.Error())
	}
	defer m.sc.CloseHandle(h)

	exitCode, err := m.sc.GetExitCodeProcess(h)
	if err != nil {
		return "", fmt.Errorf("GetExitCodeProcess: %s", err.Error())
	}

	if exitCode != STILL_ACTIVE_EXIT_CODE {
		return "stopped", nil
	}

	creationTime, err := m.sc.GetProcessStartTime(h)
	if err != nil {
		return "", fmt.Errorf("GetProcessStartTime: %s", err.Error())
	}

	if creationTime == state.StartTime {
		return "running", err
	}

	return "stopped", nil
}

func stateValid(state State) bool {
	return (state.PID == 0 && state.StartTime == syscall.Filetime{}) ||
		(state.PID != 0 && state.StartTime != syscall.Filetime{})
}

func (m *Manager) stateDir() string {
	return filepath.Join(m.rootDir, m.containerId)
}

func (m *Manager) loadState() (State, error) {
	contents, err := ioutil.ReadFile(filepath.Join(m.stateDir(), stateFile))
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(contents, &state); err != nil {
		return State{}, err
	}

	return state, nil
}

func (m *Manager) writeState(state State) error {
	contents, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(m.stateDir(), stateFile), contents, 0644)
}
