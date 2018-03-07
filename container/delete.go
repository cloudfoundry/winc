package container

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/winc/hcs"
	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

func (m *Manager) Delete(force bool) error {
	_, err := m.hcsClient.GetContainerProperties(m.id)
	if err != nil {
		if force {
			_, ok := err.(*hcs.NotFoundError)
			if ok {
				return nil
			}
		}

		return err
	}

	query := hcsshim.ComputeSystemQuery{Owners: []string{m.id}}
	sidecardContainerProperties, err := m.hcsClient.GetContainers(query)
	if err != nil {
		return err
	}
	containerIdsToDelete := []string{}
	for _, sidecardContainerProperty := range sidecardContainerProperties {
		containerIdsToDelete = append(containerIdsToDelete, sidecardContainerProperty.ID)
	}
	containerIdsToDelete = append(containerIdsToDelete, m.id)

	var errors []string
	for _, containerIdToDelete := range containerIdsToDelete {
		pid, err := m.processManager.ContainerPid(containerIdToDelete)
		if err != nil {
			logrus.Error(err.Error())
			errors = append(errors, err.Error())
			continue
		}

		err = m.mounter.Unmount(pid)
		if err != nil {
			logrus.Error(err.Error())
			errors = append(errors, err.Error())
		}

		container, err := m.hcsClient.OpenContainer(containerIdToDelete)
		if err != nil {
			logrus.Error(err.Error())
			errors = append(errors, err.Error())
			continue
		}

		err = m.deleteContainer(containerIdToDelete, container)
		if err != nil {
			logrus.Error(err.Error())
			errors = append(errors, err.Error())
			continue
		}
	}

	if len(errors) == 0 {
		return nil
	} else {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}
}
