package main_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/text/encoding/unicode"

	. "code.cloudfoundry.org/winc/cmd/winc"
	"code.cloudfoundry.org/winc/sandbox"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Validate", func() {
	var (
		logger      *logrus.Entry
		containerId string
	)

	BeforeEach(func() {
		containerId = filepath.Base(bundlePath)

		logger = logrus.WithField("suite", "winc")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
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
			var (
				expectedSpec specs.Spec
			)

			BeforeEach(func() {
				expectedSpec = runtimeSpecGenerator(sandbox.ImageSpec{
					RootFs: rootfsPath,
					Spec: specs.Spec{
						Windows: &specs.Windows{
							LayerFolders: []string{"a layer", "another layer"},
						},
					},
				}, containerId)
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
				Expect(os.RemoveAll(bundlePath)).To(Succeed())
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

			It("logs the invalid fields", func() {
				logOutputStr := logOutput.String()
				Expect(logOutputStr).To(ContainSubstring("'Spec.Version' should not be empty."))
			})
		})
	})

	Context("Process", func() {
		var (
			spec                   *specs.Process
			err                    error
			processConfig          string
			processConfigOverrides *specs.Process
		)

		BeforeEach(func() {
			f, err := ioutil.TempFile("", "process.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(f.Close()).To(Succeed())
			processConfig = f.Name()
			processConfigOverrides = &specs.Process{}
		})

		AfterEach(func() {
			Expect(os.RemoveAll(processConfig)).To(Succeed())
		})

		JustBeforeEach(func() {
			spec, err = ValidateProcess(logger, processConfig, processConfigOverrides)
		})

		Context("when provided a valid process config file", func() {
			var expectedSpec specs.Process

			BeforeEach(func() {
				expectedSpec = processSpecGenerator()
				config, err := json.Marshal(&expectedSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(processConfig, config, 0666)).To(Succeed())
			})

			It("returns the expected process spec", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(spec).To(Equal(&expectedSpec))
			})

			Context("when overrides are specified", func() {
				BeforeEach(func() {
					processConfigOverrides = &specs.Process{
						Cwd:  "C:\\foo\\bar\\baz",
						Args: []string{"foo.exe", "arg"},
						Env:  []string{"var1=foo", "var2=bar"},
						User: specs.User{
							Username: "user1",
						},
					}
				})

				It("the process config file values are overriden", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(spec.Cwd).To(Equal(processConfigOverrides.Cwd))
					Expect(spec.Args).To(Equal(processConfigOverrides.Args))
					Expect(spec.Env).To(Equal(processConfigOverrides.Env))
					Expect(spec.User.Username).To(Equal(processConfigOverrides.User.Username))
				})
			})
		})

		Context("when the process config file is not provided", func() {
			BeforeEach(func() {
				processConfigOverrides = &specs.Process{
					Cwd:  "C:\\foo\\bar\\baz",
					Args: []string{"foo.exe", "arg"},
					Env:  []string{"var1=foo", "var2=bar"},
					User: specs.User{
						Username: "user1",
					},
				}

				Expect(os.RemoveAll(processConfig)).To(Succeed())
				processConfig = ""
			})

			It("uses the overrides", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(spec).To(Equal(processConfigOverrides))
			})

			Context("when the overrides do not specify a cwd", func() {
				BeforeEach(func() {
					processConfigOverrides.Cwd = ""
				})

				It("defaults the cwd to C:\\", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(spec.Cwd).To(Equal("C:\\"))
				})
			})

			Context("when the overrides do not specify required values or specify invalid values", func() {
				var logOutput *bytes.Buffer

				BeforeEach(func() {
					processConfigOverrides = &specs.Process{
						Cwd: "foo/bar",
						Env: []string{"var1"},
					}

					logOutput = &bytes.Buffer{}
					logrus.SetOutput(logOutput)
				})

				It("errors", func() {
					Expect(err).To(MatchError(&ProcessConfigValidationError{processConfigOverrides}))
					Expect(spec).To(BeNil())
				})

				It("logs the invalid fields", func() {
					logOutputStr := logOutput.String()
					Expect(logOutputStr).To(ContainSubstring(`processConfigError="cwd \"foo/bar\" is not an absolute path"`))
					Expect(logOutputStr).To(ContainSubstring(`processConfigError="args must not be empty"`))
					Expect(logOutputStr).To(ContainSubstring(`processConfigError="env \"var1\" should be in the form of 'key=value'`))
				})
			})
		})

		Context("when the process config file does not exist", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(processConfig)).To(Succeed())
			})

			It("errors", func() {
				Expect(err).To(MatchError(&MissingProcessConfigError{processConfig}))
				Expect(spec).To(BeNil())
			})
		})

		Context("when the process config file is not UTF-8 encoded", func() {
			BeforeEach(func() {
				var spec specs.Process
				config, err := json.Marshal(&spec)
				Expect(err).ToNot(HaveOccurred())
				encoding := unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM)
				encoder := encoding.NewEncoder()
				configUnicode, err := encoder.Bytes(config)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(processConfig, configUnicode, 0666)).To(Succeed())
			})

			It("errors", func() {
				Expect(err).To(MatchError(&ProcessConfigInvalidEncodingError{processConfig}))
				Expect(spec).To(BeNil())
			})
		})

		Context("when the process config file is not valid JSON", func() {
			BeforeEach(func() {
				config := []byte("{")
				Expect(ioutil.WriteFile(processConfig, config, 0666)).To(Succeed())
			})

			It("errors", func() {
				Expect(err).To(MatchError(&ProcessConfigInvalidJSONError{processConfig}))
				Expect(spec).To(BeNil())
			})
		})

		Context("when the process config file does not conform to the runtime spec", func() {
			var (
				logOutput   *bytes.Buffer
				invalidSpec *specs.Process
			)

			BeforeEach(func() {
				invalidSpec = &specs.Process{
					Cwd: "foo/bar",
					Env: []string{"var1"},
				}

				config, err := json.Marshal(&invalidSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(processConfig, config, 0666)).To(Succeed())

				logOutput = &bytes.Buffer{}
				logrus.SetOutput(logOutput)
			})

			It("errors", func() {
				Expect(err).To(MatchError(&ProcessConfigValidationError{invalidSpec}))
				Expect(spec).To(BeNil())
			})

			It("logs the invalid fields", func() {
				logOutputStr := logOutput.String()
				Expect(logOutputStr).To(ContainSubstring(`processConfigError="cwd \"foo/bar\" is not an absolute path"`))
				Expect(logOutputStr).To(ContainSubstring(`processConfigError="args must not be empty"`))
				Expect(logOutputStr).To(ContainSubstring(`processConfigError="env \"var1\" should be in the form of 'key=value'`))
			})
		})
	})
})
