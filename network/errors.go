package network

import "fmt"

type NoNATNetworkError struct {
	Name string
}

func (e *NoNATNetworkError) Error() string {
	return fmt.Sprintf("could not load nat network: %s", e.Name)
}
