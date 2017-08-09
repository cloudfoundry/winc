package hcs

import "fmt"

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

type MissingVolumePathError struct {
	Id string
}

func (e *MissingVolumePathError) Error() string {
	return fmt.Sprintf("could not get volume path from layer: %s", e.Id)
}
