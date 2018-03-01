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
	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete", func() {
	var (
		containerId      string
		bundlePath       string
		hcsClient        *fakes.HCSClient
		mounter          *fakes.Mounter
		fakeContainer    *hcsfakes.Container
		stateManager     *fakes.StateManager
		containerManager *container.Manager
		rootDir          string
	)

	BeforeEach(func() {
		var err error
		bundlePath, err = ioutil.TempDir("", "bundlePath")
		Expect(err).ToNot(HaveOccurred())

		containerId = filepath.Base(bundlePath)

		rootDir, err = ioutil.TempDir("", "delete.root")
		Expect(err).ToNot(HaveOccurred())

		stateDir := filepath.Join(rootDir, containerId)
		Expect(os.MkdirAll(stateDir, 0755)).To(Succeed())

		hcsClient = &fakes.HCSClient{}
		mounter = &fakes.Mounter{}
		stateManager = &fakes.StateManager{}
		fakeContainer = &hcsfakes.Container{}

		logger := (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "delete")

		containerManager = container.NewManager(logger, hcsClient, mounter, stateManager, containerId, rootDir)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
		Expect(os.RemoveAll(rootDir)).To(Succeed())
	})

	Context("when the specified container is running", func() {
		var pid int
		BeforeEach(func() {
			pid = 42
			fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{
				{ProcessId: uint32(pid), ImageName: "wininit.exe"},
			}, nil)
			hcsClient.OpenContainerReturns(fakeContainer, nil)
			stateManager.ContainerPidReturnsOnCall(0, pid, nil)
		})

		It("deletes it", func() {
			Expect(containerManager.Delete(false)).To(Succeed())

			Expect(stateManager.ContainerPidCallCount()).To(Equal(1))
			Expect(stateManager.ContainerPidArgsForCall(0)).To(Equal(containerId))

			Expect(mounter.UnmountCallCount()).To(Equal(1))
			Expect(mounter.UnmountArgsForCall(0)).To(Equal(pid))

			Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
			Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

			Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(2))
			Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))
			Expect(hcsClient.GetContainerPropertiesArgsForCall(1)).To(Equal(containerId))

			Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
			Expect(filepath.Join(rootDir, containerId)).NotTo(BeADirectory())
		})

		Context("when the specified container has a sidecar", func() {
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

				stateManager.ContainerPidReturnsOnCall(0, sidecarPid, nil)
				stateManager.ContainerPidReturnsOnCall(1, pid, nil)
			})
			It("deletes the sidecar container", func() {
				hcsClient.OpenContainerReturnsOnCall(0, fakeSidecar, nil)
				hcsClient.OpenContainerReturnsOnCall(1, fakeContainer, nil)

				Expect(containerManager.Delete(false)).To(Succeed())

				Expect(hcsClient.GetContainersCallCount()).To(Equal(1))
				query := hcsshim.ComputeSystemQuery{Owners: []string{containerId}}
				Expect(hcsClient.GetContainersArgsForCall(0)).To(Equal(query))

				Expect(stateManager.ContainerPidCallCount()).To(Equal(2))
				Expect(stateManager.ContainerPidArgsForCall(0)).To(Equal(sidecarId))
				Expect(stateManager.ContainerPidArgsForCall(1)).To(Equal(containerId))

				Expect(mounter.UnmountCallCount()).To(Equal(2))
				Expect(mounter.UnmountArgsForCall(0)).To(Equal(sidecarPid))
				Expect(mounter.UnmountArgsForCall(1)).To(Equal(pid))

				Expect(hcsClient.OpenContainerCallCount()).To(Equal(2))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(sidecarId))
				Expect(hcsClient.OpenContainerArgsForCall(1)).To(Equal(containerId))

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
			Context("when we fail to unmount the sidecard container", func() {
				var unmountError error = errors.New("failed to unmount container")
				It("continue to delete the main container", func() {
					hcsClient.OpenContainerReturnsOnCall(0, fakeSidecar, nil)
					hcsClient.OpenContainerReturnsOnCall(1, fakeContainer, nil)
					mounter.UnmountReturnsOnCall(0, unmountError)
					Expect(containerManager.Delete(false)).To(Equal(unmountError))
					Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				})
			})
		})

		Context("when unmounting the sandbox fails", func() {
			BeforeEach(func() {
				mounter.UnmountReturns(errors.New("unmounting failed"))
			})

			It("continues deleting the container and returns an error", func() {
				Expect(containerManager.Delete(false)).NotTo(Succeed())

				Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

				Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
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
