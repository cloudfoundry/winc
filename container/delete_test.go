package container_test

import (
	"errors"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/containerfakes"
	"code.cloudfoundry.org/winc/hcsclient/hcsclientfakes"
	"code.cloudfoundry.org/winc/sandbox/sandboxfakes"
	"github.com/Microsoft/hcsshim"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete", func() {
	const (
		expectedContainerId        = "containerid"
		expectedContainerBundleDir = "C:\\bundle"
	)

	var (
		hcsClient        *hcsclientfakes.FakeClient
		sandboxManager   *sandboxfakes.FakeSandboxManager
		fakeContainer    *containerfakes.FakeHCSContainer
		containerManager container.ContainerManager
		expectedQuery    hcsshim.ComputeSystemQuery
	)

	BeforeEach(func() {
		hcsClient = &hcsclientfakes.FakeClient{}
		sandboxManager = &sandboxfakes.FakeSandboxManager{}
		fakeContainer = &containerfakes.FakeHCSContainer{}
		containerManager = container.NewManager(hcsClient, sandboxManager, expectedContainerId)

		expectedQuery = hcsshim.ComputeSystemQuery{
			IDs:    []string{expectedContainerId},
			Owners: []string{"winc"},
		}
	})

	Context("when the specified container is not running", func() {
		BeforeEach(func() {
			hcsClient.OpenContainerReturns(fakeContainer, nil)
		})

		It("deletes it", func() {
			Expect(containerManager.Delete()).To(Succeed())

			Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
			Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(expectedContainerId))

			Expect(fakeContainer.TerminateCallCount()).To(Equal(1))

			Expect(hcsClient.IsPendingCallCount()).To(Equal(1))
			Expect(hcsClient.IsPendingArgsForCall(0)).To(BeNil())

			Expect(sandboxManager.DeleteCallCount()).To(Equal(1))
		})

		Context("when terminating the container does not immediately succeed", func() {
			var terminateContainerError = errors.New("terminate container failed")

			BeforeEach(func() {
				hcsClient.OpenContainerReturns(fakeContainer, nil)
				fakeContainer.TerminateReturns(terminateContainerError)
				hcsClient.IsPendingReturns(false)
			})

			It("errors", func() {
				Expect(containerManager.Delete()).To(Equal(terminateContainerError))
			})

			Context("when termination is pending", func() {
				BeforeEach(func() {
					hcsClient.IsPendingReturns(true)
				})

				It("waits for termination to finish", func() {
					Expect(containerManager.Delete()).To(Succeed())
				})

				Context("when termination does not finish before the timeout", func() {
					var terminateWaitError = errors.New("waiting for terminate failed")

					BeforeEach(func() {
						fakeContainer.WaitTimeoutReturns(terminateWaitError)
					})

					It("errors", func() {
						Expect(containerManager.Delete()).To(Equal(terminateWaitError))
					})
				})
			})
		})

		Context("when the sandbox delete fails", func() {
			var deleteSandboxError = errors.New("delete sandbox failed")

			BeforeEach(func() {
				sandboxManager.DeleteReturns(deleteSandboxError)
			})

			It("errors", func() {
				Expect(containerManager.Delete()).To(Equal(deleteSandboxError))
			})
		})
	})

	XContext("when the specified container is running", func() {

	})

	Context("when the container does not exist", func() {
		var openContainerError = errors.New("open container failed")

		BeforeEach(func() {
			hcsClient.OpenContainerReturns(nil, openContainerError)
		})

		It("errors", func() {
			Expect(containerManager.Delete()).To(Equal(openContainerError))
		})
	})
})
