package main_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"golang.org/x/text/encoding/unicode"

	. "code.cloudfoundry.org/winc/cmd/winc"
	"github.com/Sirupsen/logrus"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Validate", func() {
	var logger *logrus.Entry

	BeforeEach(func() {
		logger = logrus.WithField("suite", "winc")
	})

	Context("Bundle", func() {
		var (
			spec *specs.Spec
			err  error
		)

		JustBeforeEach(func() {
			spec, err = ValidateBundle(logger, bundlePath)
		})

		Context("given a valid bundle", func() {
			var expectedSpec specs.Spec

			BeforeEach(func() {
				expectedSpec = runtimeSpecGenerator(rootfsPath)
				config, err := json.Marshal(&expectedSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0666)).To(Succeed())
			})

			It("validates the bundle and returns the expected runtime spec", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(spec).To(Equal(&expectedSpec))
			})
		})

		Context("when provided a nonexistent bundle directory", func() {
			BeforeEach(func() {
				bundlePath = "doesntexist"
			})

			It("errors", func() {
				Expect(err).To(MatchError(&MissingBundleError{BundlePath: bundlePath}))
				Expect(spec).To(BeNil())
			})
		})

		Context("when the bundle directory does not contain a config.json", func() {
			It("errors", func() {
				Expect(err).To(MatchError(&MissingBundleConfigError{BundlePath: bundlePath}))
				Expect(spec).To(BeNil())
			})
		})

		Context("when the bundle config.json is not UTF-8 encoded", func() {
			BeforeEach(func() {
				var spec specs.Spec
				config, err := json.Marshal(&spec)
				Expect(err).ToNot(HaveOccurred())
				encoding := unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM)
				encoder := encoding.NewEncoder()
				configUnicode, err := encoder.Bytes(config)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), configUnicode, 0666)).To(Succeed())
			})

			It("errors", func() {
				Expect(err).To(MatchError(&BundleConfigInvalidEncodingError{BundlePath: bundlePath}))
				Expect(spec).To(BeNil())
			})
		})

		Context("when provided a bundle with a config.json that is invalid JSON", func() {
			BeforeEach(func() {
				config := []byte("{")
				Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0666)).To(Succeed())
			})

			It("errors", func() {
				Expect(err).To(MatchError(&BundleConfigInvalidJSONError{BundlePath: bundlePath}))
				Expect(spec).To(BeNil())
			})
		})

		Context("when provided a bundle with a config.json that does not conform to the runtime spec", func() {
			var logOutput *bytes.Buffer

			BeforeEach(func() {
				var spec specs.Spec
				config, err := json.Marshal(&spec)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0666)).To(Succeed())

				logOutput = &bytes.Buffer{}
				logrus.SetOutput(logOutput)
			})

			It("errors", func() {
				Expect(err).To(MatchError(&BundleConfigValidationError{BundlePath: bundlePath}))
				Expect(spec).To(BeNil())
			})

			It("logs the errors in the config.json", func() {
				Expect(logOutput).To(ContainSubstring("'Platform.OS' should not be empty."))
				Expect(logOutput).To(ContainSubstring("'Platform.Arch' should not be empty."))
				Expect(logOutput).To(ContainSubstring("'Root.Path' should not be empty."))
			})
		})
	})
})
