package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Sirupsen/logrus"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/validate"
)

func ValidateBundle(logger *logrus.Entry, bundlePath string) (*specs.Spec, error) {
	logger.Debug("validating bundle")

	if _, err := os.Stat(bundlePath); err != nil {
		return nil, &MissingBundleError{BundlePath: bundlePath}
	}

	configPath := filepath.Join(bundlePath, specConfig)
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, &MissingBundleConfigError{BundlePath: bundlePath}
	}
	if !utf8.Valid(content) {
		return nil, &BundleConfigInvalidEncodingError{BundlePath: bundlePath}
	}
	var spec specs.Spec
	if err = json.Unmarshal(content, &spec); err != nil {
		return nil, &BundleConfigInvalidJSONError{BundlePath: bundlePath}
	}

	validator := validate.NewValidator(&spec, bundlePath, true)

	msgs := validator.CheckMandatoryFields()
	if len(msgs) != 0 {
		for _, m := range msgs {
			logger.WithField("bundleConfigError", m).Error(fmt.Sprintf("error in bundle %s", specConfig))
		}
		return nil, &BundleConfigValidationError{BundlePath: bundlePath}
	}

	return &spec, nil
}

func ValidateProcess(logger *logrus.Entry, processConfig string, overrides *specs.Process) (*specs.Process, error) {
	logger.Debug("validating process config")

	msgs := []string{}

	var spec specs.Process

	if processConfig != "" {
		content, err := ioutil.ReadFile(processConfig)
		if err != nil {
			return nil, &MissingProcessConfigError{ProcessConfig: processConfig}
		}
		if !utf8.Valid(content) {
			return nil, &ProcessConfigInvalidEncodingError{ProcessConfig: processConfig}
		}
		if err = json.Unmarshal(content, &spec); err != nil {
			return nil, &ProcessConfigInvalidJSONError{ProcessConfig: processConfig}
		}
	}

	if overrides != nil {
		if overrides.Cwd != "" {
			spec.Cwd = overrides.Cwd
		}

		if len(overrides.Args) > 0 {
			spec.Args = overrides.Args
		}

		if len(overrides.Env) > 0 {
			spec.Env = overrides.Env
		}

		if overrides.User.Username != "" {
			spec.User.Username = overrides.User.Username
		}
	}

	if !filepath.IsAbs(spec.Cwd) {
		msgs = append(msgs, fmt.Sprintf("cwd %q is not an absolute path", spec.Cwd))
	}

	if len(spec.Args) == 0 {
		msgs = append(msgs, fmt.Sprintf("args must not be empty"))
	}

	for _, env := range spec.Env {
		if !envValid(env) {
			msgs = append(msgs, fmt.Sprintf("env %q should be in the form of 'key=value'. The left hand side must consist solely of letters, digits, and underscores '_'.", env))
		}
	}

	if len(msgs) > 0 {
		for _, m := range msgs {
			logger.WithField("processConfigError", m).Error("error in process config")
		}
		return nil, &ProcessConfigValidationError{ProcessSpec: &spec}
	}

	return &spec, nil
}

func envValid(env string) bool {
	items := strings.Split(env, "=")
	if len(items) < 2 {
		return false
	}
	for i, ch := range strings.TrimSpace(items[0]) {
		if !unicode.IsDigit(ch) && !unicode.IsLetter(ch) && ch != '_' {
			return false
		}
		if i == 0 && unicode.IsDigit(ch) {
			logrus.Warnf("Env %v: variable name beginning with digit is not recommended.", env)
		}
	}
	return true
}
