package container

import "fmt"

type AlreadyExistsError struct {
	Id string
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("container with id already exists: %s", e.Id)
}

type InvalidIdError struct {
	Id string
}

func (e *InvalidIdError) Error() string {
	return fmt.Sprintf("container id does not match bundle directory name: %s", e.Id)
}

type MissingVolumePathError struct {
	Id string
}

func (e *MissingVolumePathError) Error() string {
	return fmt.Sprintf("could not get volume path for container: %s", e.Id)
}

type CouldNotCreateProcessError struct {
	Id      string
	Command string
}

func (e *CouldNotCreateProcessError) Error() string {
	return fmt.Sprintf("could not start command '%s' in container: %s", e.Command, e.Id)
}
