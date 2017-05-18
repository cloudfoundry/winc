package container_test

import (
	"errors"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient/hcsclientfakes"
	"code.cloudfoundry.org/winc/sandbox/sandboxfakes"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("State", func() {
	const (
		expectedContainerId        = "containerid"
		expectedContainerBundleDir = "C:\\bundle"
	)
	var (
		hcsClient        *hcsclientfakes.FakeClient
		sandboxManager   *sandboxfakes.FakeSandboxManager
		containerManager container.ContainerManager
	)

	BeforeEach(func() {
		hcsClient = &hcsclientfakes.FakeClient{}
		sandboxManager = &sandboxfakes.FakeSandboxManager{}
		containerManager = container.NewManager(hcsClient, sandboxManager, expectedContainerId)
	})

	It("calls the client with the correct container id", func() {
		containerManager.State()
		Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
		Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(expectedContainerId))
	})

	Context("when the specified container exists", func() {
		var (
			expectedState               *specs.State
			actualState                 *specs.State
			expectedContainerProperties hcsshim.ContainerProperties
		)

		BeforeEach(func() {
			expectedState = &specs.State{
				Version: specs.Version,
				ID:      expectedContainerId,
				Bundle:  expectedContainerBundleDir,
			}

			expectedContainerProperties = hcsshim.ContainerProperties{
				ID:   expectedContainerId,
				Name: expectedContainerBundleDir,
			}
		})

		JustBeforeEach(func() {
			var err error
			hcsClient.GetContainerPropertiesReturns(expectedContainerProperties, nil)
			actualState, err = containerManager.State()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the container has been created", func() {
			BeforeEach(func() {
				expectedState.Status = "created"
			})

			It("returns the correct state", func() {
				Expect(actualState).To(Equal(expectedState))
			})
		})

		XContext("when the container is running", func() {
			BeforeEach(func() {
				expectedState.Status = "running"
			})

			It("returns the correct state", func() {
				Expect(actualState).To(Equal(expectedState))
			})
		})

		Context("when the container has been stopped", func() {
			BeforeEach(func() {
				expectedState.Status = "stopped"
				expectedContainerProperties.Stopped = true
			})

			It("returns the correct state", func() {
				Expect(actualState).To(Equal(expectedState))
			})
		})
	})

	Context("when the specified container does not exist", func() {
		var missingContainerError = errors.New("container does not exist")

		BeforeEach(func() {
			hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{}, missingContainerError)
		})

		It("errors", func() {
			_, err := containerManager.State()
			Expect(err).To(Equal(missingContainerError))
		})
	})
})
