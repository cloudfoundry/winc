package container

import (
	"fmt"

	"code.cloudfoundry.org/winc/container/state"
)

func (m *Manager) Start() error {
	ociState, err := m.State()
	if err != nil {
		return err
	}

	if ociState.Status != "created" {
		return fmt.Errorf("cannot start a container in the %s state", ociState.Status)
	}

	spec, err := m.loadBundle(ociState.Bundle)
	if err != nil {
		return err
	}

	containerState := state.ContainerState{Bundle: ociState.Bundle}
	writeContainerState := func() error {
		return m.stateManager.WriteContainerState(containerState)
	}

	proc, err := m.Exec(spec.Process, false)
	if err != nil {
		if err2 := m.stateManager.SetExecFailed(); err2 != nil {
			return err2
		}
		return err
	}
	defer proc.Close()

	containerState.UserProgramPID = proc.Pid()

	// trying to open the process to get a handle + its start time should be valid
	// here, since the hcsshim.process struct has an open handle to the process,
	// and the PID will not be reused until all open handles are closed.
	//
	// https://blogs.msdn.microsoft.com/oldnewthing/20110107-00/?p=11803

	containerState.UserProgramStartTime, err = m.processManager.ProcessStartTime(uint32(proc.Pid()))
	if err != nil {
		writeContainerState()
		return err
	}

	return writeContainerState()
}
