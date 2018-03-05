package container_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/fakes"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Exec", func() {
	var (
		containerId      string
		bundlePath       string
		hcsClient        *fakes.HCSClient
		mounter          *fakes.Mounter
		stateManager     *fakes.StateManager
		processManager   *fakes.ProcessManager
		containerManager *container.Manager
		fakeContainer    *hcsfakes.Container
		processSpec      specs.Process
	)

	BeforeEach(func() {
		var err error
		bundlePath, err = ioutil.TempDir("", "bundlePath")
		Expect(err).ToNot(HaveOccurred())

		containerId = filepath.Base(bundlePath)

		hcsClient = &fakes.HCSClient{}
		mounter = &fakes.Mounter{}
		stateManager = &fakes.StateManager{}
		processManager = &fakes.ProcessManager{}
		fakeContainer = &hcsfakes.Container{}

		logger := (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "exec")

		containerManager = container.NewManager(logger, hcsClient, mounter, stateManager, containerId, "", processManager)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Context("when the specified container exists", func() {
		var expectedProcessConfig *hcsshim.ProcessConfig

		BeforeEach(func() {
			hcsClient.OpenContainerReturns(fakeContainer, nil)
			commandArgs := []string{"powershell.exe", "Write-Host 'hi'"}
			processSpec = specs.Process{
				Args: commandArgs,
				Cwd:  "C:\\",
				User: specs.User{
					Username: "someuser",
				},
				Env: []string{"a=b", "c=d"},
			}
			expectedProcessConfig = &hcsshim.ProcessConfig{
				CommandLine:      `powershell.exe "Write-Host 'hi'"`,
				CreateStdInPipe:  true,
				CreateStdErrPipe: true,
				CreateStdOutPipe: true,
				WorkingDirectory: processSpec.Cwd,
				User:             processSpec.User.Username,
				Environment:      map[string]string{"a": "b", "c": "d"},
			}
		})

		It("starts a process in the container", func() {
			_, err := containerManager.Exec(&processSpec, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
			Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))
			Expect(fakeContainer.CreateProcessCallCount()).To(Equal(1))
			Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
		})

		Context("when io pipes are not desired", func() {
			BeforeEach(func() {
				expectedProcessConfig.CreateStdErrPipe = false
				expectedProcessConfig.CreateStdInPipe = false
				expectedProcessConfig.CreateStdOutPipe = false
			})

			It("creates a process with no io pipes", func() {
				_, err := containerManager.Exec(&processSpec, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
			})
		})

		Context("when a command and arguments contain spaces", func() {
			It("quotes the argument", func() {
				commandArgs := []string{"command with spaces.exe", "arg with spaces", "other arg"}
				processSpec = specs.Process{
					Args: commandArgs,
				}
				expectedProcessConfig = &hcsshim.ProcessConfig{
					CommandLine:      `"command with spaces.exe" "arg with spaces" "other arg"`,
					CreateStdInPipe:  true,
					CreateStdErrPipe: true,
					CreateStdOutPipe: true,
					Environment:      map[string]string{},
				}

				_, err := containerManager.Exec(&processSpec, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
			})
		})

		Context("when a command argument is empty", func() {
			It("quotes the argument", func() {
				commandArgs := []string{"command.exe", "", ""}
				processSpec = specs.Process{
					Args: commandArgs,
				}
				expectedProcessConfig = &hcsshim.ProcessConfig{
					CommandLine:      `command.exe "" ""`,
					CreateStdInPipe:  true,
					CreateStdErrPipe: true,
					CreateStdOutPipe: true,
					Environment:      map[string]string{},
				}

				_, err := containerManager.Exec(&processSpec, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
			})
		})

		Context("when a command has no arguments", func() {
			It("quotes the argument", func() {
				commandArgs := []string{"command.exe"}
				processSpec = specs.Process{
					Args: commandArgs,
				}
				expectedProcessConfig = &hcsshim.ProcessConfig{
					CommandLine:      `command.exe`,
					CreateStdInPipe:  true,
					CreateStdErrPipe: true,
					CreateStdOutPipe: true,
					Environment:      map[string]string{},
				}

				_, err := containerManager.Exec(&processSpec, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
			})
		})

		Context("when a command has a unix path", func() {
			It("converts it to a windows path, adding .exe if there is no extension", func() {
				commandArgs := []string{"/path/to/command"}
				processSpec = specs.Process{
					Args: commandArgs,
				}
				expectedProcessConfig = &hcsshim.ProcessConfig{
					CommandLine:      `\path\to\command.exe`,
					CreateStdInPipe:  true,
					CreateStdErrPipe: true,
					CreateStdOutPipe: true,
					Environment:      map[string]string{},
				}

				_, err := containerManager.Exec(&processSpec, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
			})
		})

		Context("when creating a process in the container fails", func() {
			var couldNotCreateProcessError *container.CouldNotCreateProcessError

			BeforeEach(func() {
				couldNotCreateProcessError = &container.CouldNotCreateProcessError{
					Id:      containerId,
					Command: "powershell.exe",
				}
				fakeContainer.CreateProcessReturns(nil, couldNotCreateProcessError)
			})

			It("errors", func() {
				p, err := containerManager.Exec(&processSpec, true)
				Expect(err).To(Equal(couldNotCreateProcessError))
				Expect(p).To(BeNil())
			})
		})
	})

	Context("when the specified container does not exist", func() {
		var missingContainerError = errors.New("container does not exist")

		BeforeEach(func() {
			hcsClient.OpenContainerReturns(&hcsfakes.Container{}, missingContainerError)
		})

		It("errors", func() {
			p, err := containerManager.Exec(&processSpec, true)
			Expect(err).To(Equal(missingContainerError))
			Expect(p).To(BeNil())
		})
	})
})
