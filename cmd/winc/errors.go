package main

import "fmt"

type MissingBundleError struct {
	BundlePath string
}

func (e *MissingBundleError) Error() string {
	return fmt.Sprintf("bundle does not exist: %s", e.BundlePath)
}

type MissingBundleConfigError struct {
	BundlePath string
}

func (e *MissingBundleConfigError) Error() string {
	return fmt.Sprintf("bundle %s does not exist: %s", specConfig, e.BundlePath)
}

type BundleConfigInvalidJSONError struct{}

func (e *BundleConfigInvalidJSONError) Error() string {
	return fmt.Sprintf("bundle %s contains invalid JSON: ", specConfig)
}

type BundleConfigValidationError struct {
	BundlePath string
}

func (e *BundleConfigValidationError) Error() string {
	return fmt.Sprintf("bundle %s is invalid: %s", specConfig, e.BundlePath)
}

type InvalidLogFormatError struct {
	Format string
}

func (e *InvalidLogFormatError) Error() string {
	return fmt.Sprintf("invalid log format %s", e.Format)
}
