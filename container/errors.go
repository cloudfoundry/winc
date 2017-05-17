package container

import "fmt"

type ContainerExistsError struct {
	Id string
}

func (e *ContainerExistsError) Error() string {
	return fmt.Sprintf("container with id already exists: %s", e.Id)
}

type ContainerNotFoundError struct {
	Id string
}

func (e *ContainerNotFoundError) Error() string {
	return fmt.Sprintf("container not found: %s", e.Id)
}

type ContainerDuplicateError struct {
	Id string
}

func (e *ContainerDuplicateError) Error() string {
	return fmt.Sprintf("multiple containers found with the same id: %s", e.Id)
}
