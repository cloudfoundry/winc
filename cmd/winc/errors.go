package main

import (
	"fmt"
	"strings"
)

type WincBundleConfigValidationError struct {
	Msgs []string
}

func (e *WincBundleConfigValidationError) Error() string {
	return fmt.Sprintf("bundle %s is invalid: %s", specConfig, strings.Join(e.Msgs, ", "))
}
