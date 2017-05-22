package main

import (
	"fmt"
	"strings"
)

type MissingBundleError struct {
	BundlePath string
}

func (e *MissingBundleError) Error() string {
	return fmt.Sprintf("bundle does not exist: %s", e.BundlePath)
}

type BundleConfigInvalidJSONError struct{}

func (e *BundleConfigInvalidJSONError) Error() string {
	return fmt.Sprintf("bundle %s contains invalid JSON: ", specConfig)
}

type BundleConfigValidationError struct {
	Msgs []string
}

func (e *BundleConfigValidationError) Error() string {
	return fmt.Sprintf("bundle %s is invalid: %s", specConfig, strings.Join(e.Msgs, ", "))
}
