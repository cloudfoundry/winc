package container_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/containerfakes"
	"code.cloudfoundry.org/winc/hcs/hcsfakes"
	"github.com/Microsoft/hcsshim"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete", func() {
	var (
		containerId      string
		bundlePath       string
		hcsClient        *containerfakes.FakeHCSClient
		mounter          *containerfakes.FakeMounter
		fakeContainer    *hcsfakes.FakeContainer
		networkManager   *containerfakes.FakeNetworkManager
		containerManager *container.Manager
	)

	BeforeEach(func() {
		var err error
		bundlePath, err = ioutil.TempDir("", "bundlePath")
		Expect(err).ToNot(HaveOccurred())

		containerId = filepath.Base(bundlePath)

		hcsClient = &containerfakes.FakeHCSClient{}
		mounter = &containerfakes.FakeMounter{}
		fakeContainer = &hcsfakes.FakeContainer{}
		networkManager = &containerfakes.FakeNetworkManager{}
		containerManager = container.NewManager(hcsClient, mounter, networkManager, "", containerId)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Context("when the specified container is not running", func() {
		var pid int
		BeforeEach(func() {
			pid = 42
			fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{
				{ProcessId: uint32(pid), ImageName: "wininit.exe"},
			}, nil)
			hcsClient.OpenContainerReturns(fakeContainer, nil)
		})

		It("deletes it", func() {
			Expect(containerManager.Delete()).To(Succeed())

			Expect(mounter.UnmountCallCount()).To(Equal(1))
			Expect(mounter.UnmountArgsForCall(0)).To(Equal(pid))

			Expect(hcsClient.OpenContainerCallCount()).To(Equal(2))
			Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

			Expect(networkManager.DeleteContainerEndpointsCallCount()).To(Equal(1))
			container := networkManager.DeleteContainerEndpointsArgsForCall(0)
			Expect(container).To(Equal(fakeContainer))

			Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
			Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))

			Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
		})

		Context("when unmounting the sandbox fails", func() {
			BeforeEach(func() {
				mounter.UnmountReturns(errors.New("unmounting failed"))
			})

			It("continues deleting the container and returns an error", func() {
				Expect(containerManager.Delete()).NotTo(Succeed())

				Expect(hcsClient.OpenContainerCallCount()).To(Equal(2))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

				Expect(networkManager.DeleteContainerEndpointsCallCount()).To(Equal(1))
				container := networkManager.DeleteContainerEndpointsArgsForCall(0)
				Expect(container).To(Equal(fakeContainer))

				Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
			})
		})

		Context("when the container was never started", func() {
			BeforeEach(func() {
				hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{Stopped: true}, nil)
			})

			It("closes the container but skips shutting down and terminating it", func() {
				Expect(containerManager.Delete()).To(Succeed())

				Expect(fakeContainer.CloseCallCount()).To(Equal(1))
				Expect(fakeContainer.ShutdownCallCount()).To(Equal(0))
				Expect(fakeContainer.TerminateCallCount()).To(Equal(0))
			})

			Context("when closing the container fails", func() {
				var closeError = errors.New("closing failed")

				BeforeEach(func() {
					fakeContainer.CloseReturns(closeError)
				})

				It("errors", func() {
					Expect(containerManager.Delete()).To(Equal(closeError))
				})
			})
		})

		Context("when shutting down the container does not immediately succeed", func() {
			var shutdownContainerError = errors.New("shutdown container failed")

			BeforeEach(func() {
				hcsClient.OpenContainerReturns(fakeContainer, nil)
				fakeContainer.ShutdownReturns(shutdownContainerError)
				hcsClient.IsPendingReturns(false)
			})

			It("calls terminate", func() {
				Expect(containerManager.Delete()).To(Succeed())
				Expect(fakeContainer.TerminateCallCount()).To(Equal(1))
			})

			Context("when shutdown is pending", func() {
				BeforeEach(func() {
					hcsClient.IsPendingReturnsOnCall(0, true)
				})

				It("waits for shutdown to finish", func() {
					Expect(containerManager.Delete()).To(Succeed())
					Expect(fakeContainer.TerminateCallCount()).To(Equal(0))
				})

				Context("when shutdown does not finish before the timeout", func() {
					var shutdownWaitError = errors.New("waiting for shutdown failed")

					BeforeEach(func() {
						fakeContainer.WaitTimeoutReturnsOnCall(0, shutdownWaitError)
					})

					It("it calls terminate", func() {
						Expect(containerManager.Delete()).To(Succeed())
						Expect(fakeContainer.TerminateCallCount()).To(Equal(1))
					})

					Context("when terminate does not immediately succeed", func() {
						var terminateContainerError = errors.New("terminate container failed")

						BeforeEach(func() {
							fakeContainer.TerminateReturns(terminateContainerError)
						})

						It("errors", func() {
							Expect(containerManager.Delete()).To(Equal(terminateContainerError))
						})

						Context("when terminate is pending", func() {
							BeforeEach(func() {
								hcsClient.IsPendingReturnsOnCall(1, true)
							})

							It("waits for terminate to finish", func() {
								Expect(containerManager.Delete()).To(Succeed())
							})

							Context("when terminate does not finish before the timeout", func() {
								var terminateWaitError = errors.New("waiting for terminate failed")

								BeforeEach(func() {
									fakeContainer.WaitTimeoutReturnsOnCall(1, terminateWaitError)
								})

								It("errors", func() {
									Expect(containerManager.Delete()).To(Equal(terminateWaitError))
								})
							})
						})
					})
				})
			})
		})
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
