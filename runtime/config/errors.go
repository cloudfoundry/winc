package config

import (
	"fmt"
)

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
	return fmt.Sprintf("bundle %s does not exist: %s", SpecConfig, e.BundlePath)
}

type MissingProcessConfigError struct {
	ProcessConfig string
}

func (e *MissingProcessConfigError) Error() string {
	return fmt.Sprintf("process config does not exist: %s", e.ProcessConfig)
}

type BundleConfigInvalidJSONError struct {
	BundlePath    string
	InternalError error
}

func (e *BundleConfigInvalidJSONError) Error() string {
	return fmt.Sprintf("bundle %s contains invalid JSON: %s: %s", SpecConfig, e.BundlePath, e.InternalError)
}

type ProcessConfigInvalidJSONError struct {
	ProcessConfig string
	InternalError error
}

func (e *ProcessConfigInvalidJSONError) Error() string {
	return fmt.Sprintf("process config contains invalid JSON: %s: %s", e.ProcessConfig, e.InternalError)
}

type BundleConfigInvalidEncodingError struct {
	BundlePath string
}

func (e *BundleConfigInvalidEncodingError) Error() string {
	return fmt.Sprintf("bundle %s not encoded in UTF-8: %s", SpecConfig, e.BundlePath)
}

type ProcessConfigInvalidEncodingError struct {
	ProcessConfig string
}

func (e *ProcessConfigInvalidEncodingError) Error() string {
	return fmt.Sprintf("process config is not encoded in UTF-8: %s", e.ProcessConfig)
}

type BundleConfigValidationError struct {
	BundlePath    string
	ErrorMessages []string
}

func (e *BundleConfigValidationError) Error() string {
	errorStr := fmt.Sprintf("bundle %s is invalid: %s:", SpecConfig, e.BundlePath)
	for _, m := range e.ErrorMessages {
		errorStr += "\n\t" + m
	}

	return errorStr
}

type ProcessConfigValidationError struct {
	ErrorMessages []string
}

func (e *ProcessConfigValidationError) Error() string {
	errorStr := "process config is invalid:"
	for _, m := range e.ErrorMessages {
		errorStr += "\n\t" + m
	}

	return errorStr
}
