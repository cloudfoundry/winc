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
