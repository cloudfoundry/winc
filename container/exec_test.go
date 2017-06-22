package container_test

import (
	"errors"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/hcsclient/hcsclientfakes"
	"code.cloudfoundry.org/winc/sandbox/sandboxfakes"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Exec", func() {
	const (
		expectedContainerId        = "containerid"
		expectedContainerBundleDir = "C:\\bundle"
	)
	var (
		hcsClient        *hcsclientfakes.FakeClient
		sandboxManager   *sandboxfakes.FakeSandboxManager
		containerManager container.ContainerManager
		fakeContainer    *hcsclientfakes.FakeContainer
		fakeProcess      *hcsclientfakes.FakeProcess
		processSpec      specs.Process
	)

	BeforeEach(func() {
		hcsClient = &hcsclientfakes.FakeClient{}
		sandboxManager = &sandboxfakes.FakeSandboxManager{}
		sandboxManager.BundlePathReturns(expectedContainerBundleDir)
		containerManager = container.NewManager(hcsClient, sandboxManager, nil, expectedContainerId)
		fakeContainer = &hcsclientfakes.FakeContainer{}
		fakeProcess = &hcsclientfakes.FakeProcess{}
	})

	Context("when the specified container exists", func() {
		var expectedProcessConfig *hcsshim.ProcessConfig

		BeforeEach(func() {
			hcsClient.OpenContainerReturns(fakeContainer, nil)
			commandArgs := []string{"powershell", "Write-Host 'hi'"}
			processSpec = specs.Process{
				Args: commandArgs,
				Cwd:  "C:\\",
				User: specs.User{
					Username: "someuser",
				},
				Env: []string{"a=b", "c=d"},
			}
			expectedProcessConfig = &hcsshim.ProcessConfig{
				CommandLine:      `powershell "Write-Host 'hi'"`,
				CreateStdInPipe:  true,
				CreateStdErrPipe: true,
				CreateStdOutPipe: true,
				WorkingDirectory: processSpec.Cwd,
				User:             processSpec.User.Username,
				Environment:      map[string]string{"a": "b", "c": "d"},
			}

			fakeProcess.PidReturns(666)
			fakeContainer.CreateProcessReturns(fakeProcess, nil)
		})

		It("starts a process in the container", func() {
			p, err := containerManager.Exec(&processSpec)
			Expect(err).ToNot(HaveOccurred())
			Expect(p.Pid()).To(Equal(666))
			Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
			Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(expectedContainerId))
			Expect(fakeContainer.CreateProcessCallCount()).To(Equal(1))
			Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
			Expect(fakeProcess.PidCallCount()).To(Equal(1))
		})

		Context("when a command and arguments contain spaces", func() {
			It("quotes the argument", func() {
				commandArgs := []string{"command with spaces", "arg with spaces", "other arg"}
				processSpec = specs.Process{
					Args: commandArgs,
				}
				expectedProcessConfig = &hcsshim.ProcessConfig{
					CommandLine:      `"command with spaces" "arg with spaces" "other arg"`,
					CreateStdInPipe:  true,
					CreateStdErrPipe: true,
					CreateStdOutPipe: true,
					Environment:      map[string]string{},
				}

				_, err := containerManager.Exec(&processSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
			})
		})

		Context("when a command argument is empty", func() {
			It("quotes the argument", func() {
				commandArgs := []string{"command", "", ""}
				processSpec = specs.Process{
					Args: commandArgs,
				}
				expectedProcessConfig = &hcsshim.ProcessConfig{
					CommandLine:      `command "" ""`,
					CreateStdInPipe:  true,
					CreateStdErrPipe: true,
					CreateStdOutPipe: true,
					Environment:      map[string]string{},
				}

				_, err := containerManager.Exec(&processSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
			})
		})

		Context("when a command has no arguments", func() {
			It("quotes the argument", func() {
				commandArgs := []string{"command"}
				processSpec = specs.Process{
					Args: commandArgs,
				}
				expectedProcessConfig = &hcsshim.ProcessConfig{
					CommandLine:      `command`,
					CreateStdInPipe:  true,
					CreateStdErrPipe: true,
					CreateStdOutPipe: true,
					Environment:      map[string]string{},
				}

				_, err := containerManager.Exec(&processSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
			})
		})

		Context("when creating a process in the container fails", func() {
			var couldNotCreateProcessError = &hcsclient.CouldNotCreateProcessError{
				Id:      expectedContainerId,
				Command: "powershell",
			}

			BeforeEach(func() {
				fakeContainer.CreateProcessReturns(nil, couldNotCreateProcessError)
			})

			It("errors", func() {
				p, err := containerManager.Exec(&processSpec)
				Expect(err).To(Equal(couldNotCreateProcessError))
				Expect(p).To(BeNil())
			})
		})
	})

	Context("when the specified container does not exist", func() {
		var missingContainerError = errors.New("container does not exist")

		BeforeEach(func() {
			hcsClient.OpenContainerReturns(&hcsclientfakes.FakeContainer{}, missingContainerError)
		})

		It("errors", func() {
			p, err := containerManager.Exec(&processSpec)
			Expect(err).To(Equal(missingContainerError))
			Expect(p).To(BeNil())
		})
	})
})
