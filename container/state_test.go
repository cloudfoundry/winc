package container_test

import (
	"errors"
	"time"

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
		fakeContainer    *hcsclientfakes.FakeContainer
	)

	BeforeEach(func() {
		hcsClient = &hcsclientfakes.FakeClient{}
		sandboxManager = &sandboxfakes.FakeSandboxManager{}
		sandboxManager.BundlePathReturns(expectedContainerBundleDir)
		containerManager = container.NewManager(hcsClient, sandboxManager, nil, expectedContainerId)
		fakeContainer = &hcsclientfakes.FakeContainer{}
		fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{
			{ProcessId: 666, ImageName: "wininit.exe"},
		}, nil)
		hcsClient.OpenContainerReturns(fakeContainer, nil)

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
				Pid:     666,
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

		Context("when the container has just been created", func() {
			BeforeEach(func() {
				expectedState.Status = "created"
			})

			It("returns the correct state", func() {
				Expect(actualState).To(Equal(expectedState))
				Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
				Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(expectedContainerId))
				Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(expectedContainerId))
				Expect(fakeContainer.ProcessListCallCount()).To(Equal(1))
			})
		})

		Context("when the container has been stopped", func() {
			BeforeEach(func() {
				expectedState.Status = "stopped"
				expectedContainerProperties.Stopped = true
			})

			It("returns the correct state", func() {
				Expect(actualState).To(Equal(expectedState))
				Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
				Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(expectedContainerId))
				Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(expectedContainerId))
				Expect(fakeContainer.ProcessListCallCount()).To(Equal(1))
			})
		})

		Context("when there are no wininit.exe processes in the container", func() {
			BeforeEach(func() {
				fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{}, nil)
			})

			It("returns 0 as the pid", func() {
				Expect(actualState.Pid).To(Equal(0))
			})
		})

		Context("when there are multiple wininit.exe processes in the container", func() {
			BeforeEach(func() {
				now := time.Now()
				fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{
					{ProcessId: 668, ImageName: "wininit.exe", CreateTimestamp: now.AddDate(0, -1, 0)},
					{ProcessId: 667, ImageName: "wininit.exe", CreateTimestamp: now.AddDate(0, -2, 0)},
					{ProcessId: 666, ImageName: "wininit.exe", CreateTimestamp: now},
				}, nil)
			})

			It("returns the pid of the oldest one as the container pid", func() {
				Expect(actualState.Pid).To(Equal(667))
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
