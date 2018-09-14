package main_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Flags", func() {
	Context("when passed a nonexistent flag", func() {
		It("prints a message saying the flag does not exist", func() {
			args := []string{"--nonexistent"}
			_, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
			Expect(err).To(HaveOccurred())
			Expect(helpers.ExitCode(err)).To(Equal(1))
			Expect(stdErr.String()).To(ContainSubstring("flag provided but not defined: -nonexistent"))
		})
	})

	Context("when passed no flags", func() {
		It("prints a help message", func() {
			args := []string{}
			stdOut, _, err := helpers.Execute(exec.Command(wincBin, args...))
			Expect(err).NotTo(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("winc.exe - Open Container Initiative runtime for Windows"))
		})
	})

	Context("when passed '--help'", func() {
		It("prints a help message", func() {
			args := []string{"--help"}
			stdOut, _, err := helpers.Execute(exec.Command(wincBin, args...))
			Expect(err).ToNot(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("winc.exe - Open Container Initiative runtime for Windows"))
		})
	})

	Context("when passed '-h'", func() {
		It("prints a help message", func() {
			args := []string{"-h"}
			stdOut, _, err := helpers.Execute(exec.Command(wincBin, args...))
			Expect(err).ToNot(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("winc.exe - Open Container Initiative runtime for Windows"))
		})
	})

	Context("when passed '--log-handle'", func() {
		var (
			logHandle string
			logR      *os.File
			logW      *os.File
			dupped    syscall.Handle
		)

		BeforeEach(func() {
			var err error

			logR, logW, err = os.Pipe()
			Expect(err).NotTo(HaveOccurred())

			self, _ := syscall.GetCurrentProcess()
			err = syscall.DuplicateHandle(self, syscall.Handle(logW.Fd()), self, &dupped, 0, true, syscall.DUPLICATE_SAME_ACCESS)
			Expect(err).NotTo(HaveOccurred())
			logW.Close()

			logHandle = strconv.FormatUint(uint64(dupped), 10)
		})

		AfterEach(func() {
			if logR != nil {
				logR.Close()
			}
		})

		It("accepts the flag and prints the --log-handle flag usage", func() {
			args := []string{"--log-handle", logHandle}
			stdOut, _, err := helpers.Execute(exec.Command(wincBin, args...))
			Expect(err).NotTo(HaveOccurred())
			Expect(stdOut.String()).To(MatchRegexp("GLOBAL OPTIONS:(.|\n)*--log-handle value"))
		})

		Context("when the winc command logs non error messages", func() {
			var (
				containerId string
				bundlePath  string
				bundleSpec  specs.Spec
			)

			BeforeEach(func() {
				var err error
				bundlePath, err = ioutil.TempDir("", "winccontainer")
				Expect(err).To(Succeed())

				containerId = filepath.Base(bundlePath)

				bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
				helpers.GenerateBundle(bundleSpec, bundlePath)
			})

			AfterEach(func() {
				helpers.DeleteContainer(containerId)
				helpers.DeleteVolume(containerId)
				Expect(os.RemoveAll(bundlePath)).To(Succeed())
			})

			It("does not log anything", func() {
				wg := &sync.WaitGroup{}
				log := new(bytes.Buffer)
				wg.Add(1)
				go streamLogs(logR, log, wg)

				args := []string{"--log-handle", logHandle, "create", containerId, "-b", bundlePath}
				stdOut, _, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(syscall.CloseHandle(dupped)).To(Succeed())
				Expect(err).NotTo(HaveOccurred())

				wg.Wait()
				Expect(log.String()).To(Equal(""))

				Expect(stdOut.String()).To(BeEmpty())
			})

			Context("when the log handle is not valid", func() {
				It("writes a useful error to stderr", func() {
					/* We hope that a sufficiently large file handle would be invalid */
					invalidFileHandle := "123456789"
					args := []string{"--log-handle", invalidFileHandle, "create", containerId, "-b", bundlePath}
					_, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
					Expect(err).To(HaveOccurred())
					Expect(stdErr.String()).To(ContainSubstring(fmt.Sprintf("log handle %s invalid: The handle is invalid.", invalidFileHandle)))
				})
			})

			Context("when the --debug flag is set", func() {
				It("logs to the log handle", func() {
					wg := &sync.WaitGroup{}
					log := new(bytes.Buffer)
					wg.Add(1)
					go streamLogs(logR, log, wg)

					args := []string{"--log-handle", logHandle, "--debug", "create", containerId, "-b", bundlePath}
					stdOut, _, err := helpers.Execute(exec.Command(wincBin, args...))
					Expect(syscall.CloseHandle(dupped)).To(Succeed())
					Expect(err).NotTo(HaveOccurred())

					wg.Wait()
					Expect(log.String()).To(ContainSubstring(fmt.Sprintf(`"containerId":"%s"`, containerId)))

					Expect(stdOut.String()).To(BeEmpty())
				})
			})
		})

		Context("when the winc command errors", func() {
			It("logs the error to the specified handle and still prints the final error to stderr", func() {
				wg := &sync.WaitGroup{}
				log := new(bytes.Buffer)
				wg.Add(1)
				go streamLogs(logR, log, wg)

				args := []string{"--log-handle", logHandle, "create", "nonexistent"}
				_, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).To(HaveOccurred())
				Expect(stdErr.String()).To(ContainSubstring("bundle config.json does not exist"))
				Expect(syscall.CloseHandle(dupped)).To(Succeed())

				wg.Wait()
				expectedLogContents := strings.Replace(strings.Trim(stdErr.String(), "\n"), `\`, `\\`, -1)
				Expect(log.String()).To(ContainSubstring(expectedLogContents))
			})
		})
	})

	Context("when passed '--log'", func() {
		var (
			logFile string
			tempDir string
		)

		BeforeEach(func() {
			var err error
			tempDir, err = ioutil.TempDir("", "log-dir")
			Expect(err).NotTo(HaveOccurred())

			logFile = filepath.Join(tempDir, "winc.log")

		})

		AfterEach(func() {
			Expect(os.RemoveAll(tempDir)).To(Succeed())
		})

		It("accepts the flag and prints the --log flag usage", func() {
			args := []string{"--log", logFile}
			stdOut, _, err := helpers.Execute(exec.Command(wincBin, args...))
			Expect(err).NotTo(HaveOccurred())
			Expect(stdOut.String()).To(MatchRegexp("GLOBAL OPTIONS:(.|\n)*--log value"))
		})

		Context("when the winc command logs non error messages", func() {
			var (
				containerId string
				bundlePath  string
				bundleSpec  specs.Spec
			)

			BeforeEach(func() {
				var err error
				bundlePath, err = ioutil.TempDir("", "winccontainer")
				Expect(err).To(Succeed())

				containerId = filepath.Base(bundlePath)

				bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
				helpers.GenerateBundle(bundleSpec, bundlePath)
			})

			AfterEach(func() {
				helpers.DeleteContainer(containerId)
				helpers.DeleteVolume(containerId)
				Expect(os.RemoveAll(bundlePath)).To(Succeed())
			})

			It("does not log anything", func() {
				args := []string{"--log", logFile, "create", containerId, "-b", bundlePath}
				stdOut, _, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).NotTo(HaveOccurred())

				log, err := ioutil.ReadFile(logFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(log).To(BeEmpty())

				Expect(stdOut.String()).To(BeEmpty())
			})

			Context("when the log file path does not exist", func() {
				It("creates it", func() {
					logFile = filepath.Join(tempDir, "something", "winc.log")
					args := []string{"--log", logFile, "create", containerId, "-b", bundlePath}
					_, _, err := helpers.Execute(exec.Command(wincBin, args...))
					Expect(err).NotTo(HaveOccurred())
					Expect(logFile).To(BeAnExistingFile())
				})
			})

			Context("when the --debug flag is set", func() {
				It("logs to the log file instead of stdout", func() {
					args := []string{"--log", logFile, "--debug", "create", containerId, "-b", bundlePath}
					stdOut, _, err := helpers.Execute(exec.Command(wincBin, args...))
					Expect(err).NotTo(HaveOccurred())

					log, err := ioutil.ReadFile(logFile)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(log)).To(ContainSubstring(fmt.Sprintf(`"containerId":"%s"`, containerId)))

					Expect(stdOut.String()).To(BeEmpty())
				})
			})
		})

		Context("when the winc command errors", func() {
			It("logs the error to the specified log file and still prints the final error to stderr", func() {
				args := []string{"--log", logFile, "create", "nonexistent"}
				_, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).To(HaveOccurred())
				Expect(stdErr.String()).To(ContainSubstring("bundle config.json does not exist"))

				log, err := ioutil.ReadFile(logFile)
				Expect(err).ToNot(HaveOccurred())

				expectedLogContents := strings.Replace(strings.Trim(stdErr.String(), "\n"), `\`, `\\`, -1)
				Expect(string(log)).To(ContainSubstring(expectedLogContents))
			})
		})
	})

	Context("when both --log-handle and --log is passed", func() {
		It("errors", func() {
			args := []string{"--log", "some-file", "--log-handle", "1234", "create", "some-container"}
			_, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
			Expect(err).To(HaveOccurred())
			Expect(stdErr.String()).To(ContainSubstring("only one of --log and --log-handle can be passed"))
		})
	})

	Context("when passed '--log-format'", func() {
		It("accepts the flag and prints the --log-format flag usage", func() {
			args := []string{"--log-format", "text"}
			stdOut, _, err := helpers.Execute(exec.Command(wincBin, args...))
			Expect(err).NotTo(HaveOccurred())
			Expect(stdOut.String()).To(MatchRegexp("GLOBAL OPTIONS:(.|\n)*--log-format value"))
		})

		Context("when provided an invalid log format", func() {
			It("errors", func() {
				args := []string{"--log-format", "invalid"}
				_, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).To(HaveOccurred())
				Expect(stdErr.String()).To(ContainSubstring("invalid log format invalid"))
			})
		})
	})

	Context("when passed '--image-store'", func() {
		var (
			containerId string
			bundlePath  string
			bundleSpec  specs.Spec
			storePath   string
		)

		BeforeEach(func() {
			var err error
			storePath, err = ioutil.TempDir("", "wincroot")
			Expect(err).ToNot(HaveOccurred())

			bundlePath, err = ioutil.TempDir("", "winccontainer")
			Expect(err).To(Succeed())

			containerId = filepath.Base(bundlePath)

			bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
			helpers.GenerateBundle(bundleSpec, bundlePath)
		})

		AfterEach(func() {
			helpers.DeleteContainer(containerId)
			helpers.DeleteVolume(containerId)
			Expect(os.RemoveAll(bundlePath)).To(Succeed())
			Expect(os.RemoveAll(storePath)).To(Succeed())
		})

		It("ignores it and creates a container successfully", func() {
			args := []string{"--image-store", storePath, "create", containerId, "-b", bundlePath}
			_, _, err := helpers.Execute(exec.Command(wincBin, args...))
			Expect(err).NotTo(HaveOccurred())
			Expect(helpers.ContainerExists(containerId)).To(BeTrue())
		})
	})

	Context("when passed '--root'", func() {
		var (
			containerId string
			bundlePath  string
			bundleSpec  specs.Spec
			rootPath    string
		)

		BeforeEach(func() {
			var err error
			rootPath, err = ioutil.TempDir("", "wincroot")
			Expect(err).ToNot(HaveOccurred())

			bundlePath, err = ioutil.TempDir("", "winccontainer")
			Expect(err).To(Succeed())

			containerId = filepath.Base(bundlePath)

			bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
			helpers.GenerateBundle(bundleSpec, bundlePath)
		})

		AfterEach(func() {
			args := []string{"--root", rootPath, "delete", containerId}
			_, _, err := helpers.Execute(exec.Command(wincBin, args...))
			Expect(err).NotTo(HaveOccurred())

			helpers.DeleteVolume(containerId)
			Expect(os.RemoveAll(bundlePath)).To(Succeed())
			Expect(os.RemoveAll(rootPath)).To(Succeed())
		})

		It("creates a state.json file in <rootPath>/<bundleId>/state.json", func() {
			args := []string{"--root", rootPath, "create", containerId, "-b", bundlePath}
			_, _, err := helpers.Execute(exec.Command(wincBin, args...))
			Expect(err).NotTo(HaveOccurred())

			jsonFile := filepath.Join(rootPath, containerId, "state.json")
			Expect(jsonFile).To(BeAnExistingFile())
		})
	})
})

func streamLogs(f *os.File, out *bytes.Buffer, wg *sync.WaitGroup) {
	defer GinkgoRecover()
	defer wg.Done()
	defer f.Close()
	_, err := io.Copy(out, f)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}
