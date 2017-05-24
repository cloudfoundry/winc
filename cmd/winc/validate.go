package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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

	m := validator.CheckMandatoryFields()
	if len(m) != 0 {
		for _, v := range m {
			logger.WithField("bundleConfigError", v).Error(fmt.Sprintf("error in bundle %s", specConfig))
		}
		return nil, &BundleConfigValidationError{BundlePath: bundlePath}
	}

	return &spec, nil
}
