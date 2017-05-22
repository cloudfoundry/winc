package container_test

import (
	"errors"
	"strings"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/containerfakes"
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
		fakeContainer    *containerfakes.FakeHCSContainer
	)
	var process specs.Process

	BeforeEach(func() {
		hcsClient = &hcsclientfakes.FakeClient{}
		sandboxManager = &sandboxfakes.FakeSandboxManager{}
		sandboxManager.BundlePathReturns(expectedContainerBundleDir)
		containerManager = container.NewManager(hcsClient, sandboxManager, expectedContainerId)
		fakeContainer = &containerfakes.FakeHCSContainer{}
	})

	Context("when the specified container exists", func() {
		var expectedProcessConfig *hcsshim.ProcessConfig

		BeforeEach(func() {
			hcsClient.OpenContainerReturns(fakeContainer, nil)
			commandArgs := []string{"powershell", "Write-Host 'hi'"}
			process = specs.Process{
				Args: commandArgs,
			}
			expectedProcessConfig = &hcsshim.ProcessConfig{
				CommandLine: strings.Join(commandArgs, " "),
			}
		})

		It("starts a process in the container", func() {
			Expect(containerManager.Exec(&process)).To(Succeed())
			Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
			Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(expectedContainerId))
			Expect(fakeContainer.CreateProcessCallCount()).To(Equal(1))
			Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
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
				Expect(containerManager.Exec(&process)).To(Equal(couldNotCreateProcessError))
			})
		})
	})

	Context("when the specified container does not exist", func() {
		var missingContainerError = errors.New("container does not exist")

		BeforeEach(func() {
			hcsClient.OpenContainerReturns(&containerfakes.FakeHCSContainer{}, missingContainerError)
		})

		It("errors", func() {
			Expect(containerManager.Exec(&process)).To(Equal(missingContainerError))
		})
	})
})
