package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/blang/semver"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/validate"
	"github.com/sirupsen/logrus"
)

const (
	SpecConfig = "config.json"
	defaultCwd = "C:\\"
)

func ValidateBundle(logger *logrus.Entry, bundlePath string) (*specs.Spec, error) {
	logger.Debug("validating bundle")

	if _, err := os.Stat(bundlePath); err != nil {
		return nil, &MissingBundleError{BundlePath: bundlePath}
	}

	configPath := filepath.Join(bundlePath, SpecConfig)
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, &MissingBundleConfigError{BundlePath: bundlePath}
	}
	if !utf8.Valid(content) {
		return nil, &BundleConfigInvalidEncodingError{BundlePath: bundlePath}
	}
	var spec specs.Spec
	if err = json.Unmarshal(content, &spec); err != nil {
		return nil, &BundleConfigInvalidJSONError{BundlePath: bundlePath, InternalError: err}
	}

	validator := validate.NewValidator(&spec, bundlePath, true, "windows")
	msgs := checkAll(spec, validator)
	if len(msgs) != 0 {
		for _, m := range msgs {
			logger.WithField("bundleConfigError", m).Error(fmt.Sprintf("error in bundle %s", SpecConfig))
		}
		return nil, &BundleConfigValidationError{BundlePath: bundlePath, ErrorMessages: msgs}
	}

	return &spec, nil
}

func checkAll(spec specs.Spec, v validate.Validator) []string {
	msgs := []string{}
	msgs = append(msgs, v.CheckPlatform()...)
	msgs = append(msgs, v.CheckMandatoryFields()...)
	msgs = append(msgs, checkSemVer(spec.Version)...)
	return msgs
}

func ValidateProcess(logger *logrus.Entry, processConfig string, overrides *specs.Process) (*specs.Process, error) {
	logger.Debug("validating process config")

	msgs := []string{}

	var spec specs.Process

	if processConfig == "" {
		spec.Cwd = defaultCwd
	} else {
		content, err := ioutil.ReadFile(processConfig)
		if err != nil {
			return nil, &MissingProcessConfigError{ProcessConfig: processConfig}
		}
		if !utf8.Valid(content) {
			return nil, &ProcessConfigInvalidEncodingError{ProcessConfig: processConfig}
		}
		if err = json.Unmarshal(content, &spec); err != nil {
			return nil, &ProcessConfigInvalidJSONError{ProcessConfig: processConfig, InternalError: err}
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

	spec.Cwd = toWindowsPath(spec.Cwd)

	if !filepath.IsAbs(spec.Cwd) {
		msgs = append(msgs, fmt.Sprintf("cwd %q is not an absolute path", spec.Cwd))
	}

	if len(spec.Args) == 0 {
		msgs = append(msgs, fmt.Sprintf("args must not be empty"))
	}

	for _, env := range spec.Env {
		if !envValid(env) {
			msgs = append(msgs, fmt.Sprintf("env %q should be in the form of 'key=value'.", env))
		}
	}

	if len(msgs) > 0 {
		for _, m := range msgs {
			logger.WithField("processConfigError", m).Error("error in process config")
		}
		return nil, &ProcessConfigValidationError{ErrorMessages: msgs}
	}

	return &spec, nil
}

func envValid(env string) bool {
	items := strings.Split(env, "=")
	if len(items) < 2 {
		return false
	}
	return true
}

func checkSemVer(version string) []string {
	logrus.Debugf("check semver")

	parsedVersion, err := semver.Parse(version)
	if err != nil {
		return []string{fmt.Sprintf("%q is not a valid SemVer: %s", version, err.Error())}
	}
	if parsedVersion.Major != uint64(specs.VersionMajor) {
		return []string{fmt.Sprintf("validate currently only handles version %d.*, but the supplied configuration targets %s", specs.VersionMajor, version)}
	}

	return []string{}
}

func toWindowsPath(input string) string {
	vol := filepath.VolumeName(input)
	if vol == "" {
		input = filepath.Join("C:", input)
	}
	return filepath.Clean(input)
}
