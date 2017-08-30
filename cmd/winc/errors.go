package main

import (
	"fmt"
)

type InvalidLogFormatError struct {
	Format string
}

func (e *InvalidLogFormatError) Error() string {
	return fmt.Sprintf("invalid log format %s", e.Format)
}
