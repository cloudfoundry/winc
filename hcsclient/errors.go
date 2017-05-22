package hcsclient

import "fmt"

type AlreadyExistsError struct {
	Id string
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("container with id already exists: %s", e.Id)
}

type NotFoundError struct {
	Id string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("container not found: %s", e.Id)
}

type DuplicateError struct {
	Id string
}

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("multiple containers found with the same id: %s", e.Id)
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
