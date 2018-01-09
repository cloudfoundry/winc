package layer

import "fmt"

type MissingVolumePathError struct {
	Id string
}

func (e *MissingVolumePathError) Error() string {
	return fmt.Sprintf("could not get volume path from layer: %s", e.Id)
}
