package config

import (
	"fmt"

	specs "github.com/opencontainers/runtime-spec/specs-go"
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
	BundlePath string
}

func (e *BundleConfigInvalidJSONError) Error() string {
	return fmt.Sprintf("bundle %s contains invalid JSON: %s", SpecConfig, e.BundlePath)
}

type ProcessConfigInvalidJSONError struct {
	ProcessConfig string
}

func (e *ProcessConfigInvalidJSONError) Error() string {
	return fmt.Sprintf("process config contains invalid JSON: %s", e.ProcessConfig)
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
	BundlePath string
}

func (e *BundleConfigValidationError) Error() string {
	return fmt.Sprintf("bundle %s is invalid: %s", SpecConfig, e.BundlePath)
}

type ProcessConfigValidationError struct {
	ProcessSpec *specs.Process
}

func (e *ProcessConfigValidationError) Error() string {
	return fmt.Sprintf("process config is invalid: %+v", e.ProcessSpec)
}
