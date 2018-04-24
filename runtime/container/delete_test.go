package container_test

import (
	"errors"
	"io/ioutil"
	"path/filepath"

	"code.cloudfoundry.org/winc/runtime/container"
	"code.cloudfoundry.org/winc/runtime/container/fakes"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete", func() {
	const containerId = "container-to-delete"
	var (
		hcsClient        *fakes.HCSClient
		fakeContainer    *hcsfakes.Container
		containerManager *container.Manager
	)

	BeforeEach(func() {
		hcsClient = &fakes.HCSClient{}
		fakeContainer = &hcsfakes.Container{}

		logger := (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "delete")

		containerManager = container.New(logger, hcsClient, containerId)
	})

	Context("when the specified container is running", func() {
		BeforeEach(func() {
			hcsClient.OpenContainerReturns(fakeContainer, nil)
		})

		It("deletes it", func() {
			Expect(containerManager.Delete(false)).To(Succeed())

			Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
			Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

			Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
			Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))

			Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
		})

		XContext("when the specified container has a sidecar", func() {
			var fakeSidecar *hcsfakes.Container
			var sidecarId string
			var sidecarPid int
			BeforeEach(func() {
				fakeSidecar = &hcsfakes.Container{}

				sidecarPid = 55
				sidecarBundlePath, err := ioutil.TempDir("", "sidecarBundlePath")
				Expect(err).ToNot(HaveOccurred())
				sidecarId = filepath.Base(sidecarBundlePath)

				hcsClient.GetContainersReturns([]hcsshim.ContainerProperties{
					hcsshim.ContainerProperties{ID: sidecarId, Owner: containerId},
				}, nil)
				fakeSidecar.ProcessListReturns([]hcsshim.ProcessListItem{
					{ProcessId: uint32(sidecarPid), ImageName: "wininit.exe"},
				}, nil)
			})
			It("deletes the sidecar container", func() {
				hcsClient.OpenContainerReturnsOnCall(0, fakeSidecar, nil)
				hcsClient.OpenContainerReturnsOnCall(1, fakeSidecar, nil)
				hcsClient.OpenContainerReturnsOnCall(2, fakeContainer, nil)
				hcsClient.OpenContainerReturnsOnCall(3, fakeContainer, nil)

				Expect(containerManager.Delete(false)).To(Succeed())

				Expect(hcsClient.GetContainersCallCount()).To(Equal(1))
				query := hcsshim.ComputeSystemQuery{Owners: []string{containerId}}
				Expect(hcsClient.GetContainersArgsForCall(0)).To(Equal(query))

				Expect(hcsClient.OpenContainerCallCount()).To(Equal(4))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(sidecarId))

				Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(3))
				Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))
				Expect(hcsClient.GetContainerPropertiesArgsForCall(1)).To(Equal(sidecarId))
				Expect(hcsClient.GetContainerPropertiesArgsForCall(2)).To(Equal(containerId))

				Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				Expect(fakeSidecar.ShutdownCallCount()).To(Equal(1))
			})
			Context("when we fail to open the sidecard container", func() {
				var openError error = errors.New("failed to open container")
				It("continue to delete the main container", func() {
					hcsClient.OpenContainerReturnsOnCall(0, nil, openError)
					hcsClient.OpenContainerReturnsOnCall(1, fakeContainer, nil)
					hcsClient.OpenContainerReturnsOnCall(2, fakeContainer, nil)
					Expect(containerManager.Delete(false)).To(Equal(openError))
					Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				})
			})
			Context("when we fail to unmount the sidecar container", func() {
				var unmountError error = errors.New("failed to unmount container")
				It("continue to delete the main container", func() {
					hcsClient.OpenContainerReturnsOnCall(0, fakeSidecar, nil)
					hcsClient.OpenContainerReturnsOnCall(1, fakeSidecar, nil)
					hcsClient.OpenContainerReturnsOnCall(2, fakeContainer, nil)
					hcsClient.OpenContainerReturnsOnCall(3, fakeContainer, nil)
					Expect(containerManager.Delete(false)).To(Equal(unmountError))
					Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				})
			})
		})

		Context("when the container was never started", func() {
			BeforeEach(func() {
				hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{Stopped: true}, nil)
			})

			It("closes the container but skips shutting down and terminating it", func() {
				Expect(containerManager.Delete(false)).To(Succeed())

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
					Expect(containerManager.Delete(false)).To(Equal(closeError))
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
				Expect(containerManager.Delete(false)).To(Succeed())
				Expect(fakeContainer.TerminateCallCount()).To(Equal(1))
			})

			Context("when shutdown is pending", func() {
				BeforeEach(func() {
					hcsClient.IsPendingReturnsOnCall(0, true)
				})

				It("waits for shutdown to finish", func() {
					Expect(containerManager.Delete(false)).To(Succeed())
					Expect(fakeContainer.TerminateCallCount()).To(Equal(0))
				})

				Context("when shutdown does not finish before the timeout", func() {
					var shutdownWaitError = errors.New("waiting for shutdown failed")

					BeforeEach(func() {
						fakeContainer.WaitTimeoutReturnsOnCall(0, shutdownWaitError)
					})

					It("it calls terminate", func() {
						Expect(containerManager.Delete(false)).To(Succeed())
						Expect(fakeContainer.TerminateCallCount()).To(Equal(1))
					})

					Context("when terminate does not immediately succeed", func() {
						var terminateContainerError = errors.New("terminate container failed")

						BeforeEach(func() {
							fakeContainer.TerminateReturns(terminateContainerError)
						})

						It("errors", func() {
							Expect(containerManager.Delete(false)).To(Equal(terminateContainerError))
						})

						Context("when terminate is pending", func() {
							BeforeEach(func() {
								hcsClient.IsPendingReturnsOnCall(1, true)
							})

							It("waits for terminate to finish", func() {
								Expect(containerManager.Delete(false)).To(Succeed())
							})

							Context("when terminate does not finish before the timeout", func() {
								var terminateWaitError = errors.New("waiting for terminate failed")

								BeforeEach(func() {
									fakeContainer.WaitTimeoutReturnsOnCall(1, terminateWaitError)
								})

								It("errors", func() {
									Expect(containerManager.Delete(false)).To(Equal(terminateWaitError))
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
			Expect(containerManager.Delete(false)).To(Equal(openContainerError))
		})
	})
})
