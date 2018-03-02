package state

import (
	"time"

	"github.com/Microsoft/hcsshim"
)

func ContainerPid(hcsClient HCSClient, id string) (int, error) {
	container, err := hcsClient.OpenContainer(id)
	if err != nil {
		return -1, err
	}
	defer container.Close()

	pl, err := container.ProcessList()
	if err != nil {
		return -1, err
	}

	var process hcsshim.ProcessListItem
	oldestTime := time.Now()
	for _, v := range pl {
		if v.ImageName == "wininit.exe" && v.CreateTimestamp.Before(oldestTime) {
			oldestTime = v.CreateTimestamp
			process = v
		}
	}

	return int(process.ProcessId), nil
}
