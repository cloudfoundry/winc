package container

import (
	"errors"
	"fmt"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func (m *Manager) State() (*specs.State, error) {
	status, bundlePath, err := m.stateManager.Get()
	if err != nil {
		return nil, err
	}

	containerPid, err := m.processManager.ContainerPid(m.id)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("container not found: %s", m.id))
	}

	return &specs.State{
		Version: specs.Version,
		Status:  status,
		Bundle:  bundlePath,
		ID:      m.id,
		Pid:     containerPid,
	}, nil
}
