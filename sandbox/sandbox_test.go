package sandbox_test

import (
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/hcsclient/hcsclientfakes"
	"code.cloudfoundry.org/winc/sandbox"
	"code.cloudfoundry.org/winc/sandbox/sandboxfakes"
	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sandbox", func() {
	const containerVolume = "containerVolume"

	var (
		storePath          string
		rootfs             string
		containerId        string
		hcsClient          *hcsclientfakes.FakeClient
		limiter            *sandboxfakes.FakeLimiter
		sandboxManager     sandbox.SandboxManager
		expectedDriverInfo hcsshim.DriverInfo
		rootfsParents      []byte
	)

	BeforeEach(func() {
		var err error
		rootfs, err = ioutil.TempDir("", "rootfs")
		Expect(err).ToNot(HaveOccurred())

		storePath, err = ioutil.TempDir("", "sandbox-store")
		Expect(err).ToNot(HaveOccurred())

		rand.Seed(time.Now().UnixNano())
		containerId = strconv.Itoa(rand.Int())

		hcsClient = &hcsclientfakes.FakeClient{}
		limiter = &sandboxfakes.FakeLimiter{}
		sandboxManager = sandbox.NewManager(hcsClient, limiter, storePath, containerId)

		expectedDriverInfo = hcsshim.DriverInfo{
			HomeDir: storePath,
			Flavour: 1,
		}
		rootfsParents = []byte(`["path1", "path2"]`)

		hcsClient.GetLayerMountPathReturns(containerVolume, nil)
	})

	JustBeforeEach(func() {
		Expect(ioutil.WriteFile(filepath.Join(rootfs, "layerchain.json"), rootfsParents, 0755)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
		Expect(os.RemoveAll(rootfs)).To(Succeed())
	})

	Context("Create", func() {
		Context("when provided a rootfs layer", func() {
			It("creates and activates the sandbox", func() {
				expectedLayerFolders := []string{rootfs, "path1", "path2"}

				actualImageSpec, err := sandboxManager.Create(rootfs, 666)
				Expect(err).ToNot(HaveOccurred())
				expectedImageSpec := &sandbox.ImageSpec{
					RootFs:       containerVolume,
					LayerFolders: expectedLayerFolders,
				}
				Expect(actualImageSpec).To(Equal(expectedImageSpec))

				Expect(hcsClient.CreateSandboxLayerCallCount()).To(Equal(1))
				driverInfo, actualContainerId, parentLayer, parentLayers := hcsClient.CreateSandboxLayerArgsForCall(0)
				Expect(driverInfo).To(Equal(expectedDriverInfo))
				Expect(actualContainerId).To(Equal(containerId))
				Expect(parentLayer).To(Equal(rootfs))
				Expect(parentLayers).To(Equal(expectedLayerFolders))

				Expect(hcsClient.ActivateLayerCallCount()).To(Equal(1))
				driverInfo, actualContainerId = hcsClient.ActivateLayerArgsForCall(0)
				Expect(driverInfo).To(Equal(expectedDriverInfo))
				Expect(actualContainerId).To(Equal(containerId))

				Expect(hcsClient.PrepareLayerCallCount()).To(Equal(1))
				driverInfo, actualContainerId, parentLayers = hcsClient.PrepareLayerArgsForCall(0)
				Expect(driverInfo).To(Equal(expectedDriverInfo))
				Expect(actualContainerId).To(Equal(containerId))
				Expect(parentLayers).To(Equal(expectedLayerFolders))

				Expect(limiter.SetDiskLimitCallCount()).To(Equal(1))
				actualContainerVolume, actualDiskLimit := limiter.SetDiskLimitArgsForCall(0)
				Expect(actualContainerVolume).To(Equal(containerVolume))
				Expect(actualDiskLimit).To(BeEquivalentTo(666))
			})

			Context("when creating the sandbox fails", func() {
				var createSandboxLayerError = errors.New("create sandbox failed")

				BeforeEach(func() {
					hcsClient.CreateSandboxLayerReturns(createSandboxLayerError)
				})

				It("errors", func() {
					_, err := sandboxManager.Create(rootfs, 666)
					Expect(err).To(Equal(createSandboxLayerError))
				})
			})

			Context("when activating the sandbox fails", func() {
				var activateLayerError = errors.New("activate sandbox failed")

				BeforeEach(func() {
					hcsClient.ActivateLayerReturns(activateLayerError)
				})

				It("errors", func() {
					_, err := sandboxManager.Create(rootfs, 666)
					Expect(err).To(Equal(activateLayerError))
				})
			})

			Context("when preparing the sandbox fails", func() {
				var prepareLayerError = errors.New("prepare sandbox failed")

				BeforeEach(func() {
					hcsClient.PrepareLayerReturns(prepareLayerError)
				})

				It("errors", func() {
					_, err := sandboxManager.Create(rootfs, 666)
					Expect(err).To(Equal(prepareLayerError))
				})
			})

			Context("when setting the disk limit fails", func() {
				var diskLimitError = errors.New("setting disk limit failed")

				BeforeEach(func() {
					limiter.SetDiskLimitReturns(diskLimitError)
					hcsClient.LayerExistsReturns(true, nil)
				})

				It("deletes the sandbox and errors", func() {
					_, err := sandboxManager.Create(rootfs, 666)
					Expect(err).To(Equal(diskLimitError))

					Expect(hcsClient.LayerExistsCallCount()).To(Equal(1))
					driverInfo, actualContainerId := hcsClient.LayerExistsArgsForCall(0)
					Expect(driverInfo).To(Equal(expectedDriverInfo))
					Expect(actualContainerId).To(Equal(containerId))

					Expect(hcsClient.UnprepareLayerCallCount()).To(Equal(1))
					driverInfo, actualContainerId = hcsClient.UnprepareLayerArgsForCall(0)
					Expect(driverInfo).To(Equal(expectedDriverInfo))
					Expect(actualContainerId).To(Equal(containerId))

					Expect(hcsClient.DeactivateLayerCallCount()).To(Equal(1))
					driverInfo, actualContainerId = hcsClient.DeactivateLayerArgsForCall(0)
					Expect(driverInfo).To(Equal(expectedDriverInfo))
					Expect(actualContainerId).To(Equal(containerId))

					Expect(hcsClient.DestroyLayerCallCount()).To(Equal(1))
					driverInfo, actualContainerId = hcsClient.DestroyLayerArgsForCall(0)
					Expect(driverInfo).To(Equal(expectedDriverInfo))
					Expect(actualContainerId).To(Equal(containerId))
				})
			})
		})

		Context("when provided a nonexistent rootfs layer", func() {
			It("errors", func() {
				_, err := sandboxManager.Create("nonexistentrootfs", 666)
				Expect(err).To(Equal(&sandbox.MissingRootfsError{Msg: "nonexistentrootfs"}))
			})
		})

		Context("when provided a rootfs layer missing a layerchain.json", func() {
			JustBeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(rootfs, "layerchain.json"))).To(Succeed())
			})

			It("errors", func() {
				_, err := sandboxManager.Create(rootfs, 666)
				Expect(err).To(Equal(&sandbox.MissingRootfsLayerChainError{Msg: rootfs}))
			})
		})

		Context("when the rootfs has a layerchain.json that is invalid JSON", func() {
			BeforeEach(func() {
				rootfsParents = []byte("[")
			})

			It("errors", func() {
				_, err := sandboxManager.Create(rootfs, 666)
				Expect(err).To(Equal(&sandbox.InvalidRootfsLayerChainError{Msg: rootfs}))
			})
		})

		Context("when getting the volume mount path of the container fails", func() {
			Context("when getting the volume returned an error", func() {
				var layerMountPathError = errors.New("could not get volume")

				BeforeEach(func() {
					hcsClient.GetLayerMountPathReturns("", layerMountPathError)
				})

				It("errors", func() {
					_, err := sandboxManager.Create(rootfs, 666)
					Expect(err).To(Equal(layerMountPathError))
				})
			})

			Context("when the volume returned is empty", func() {
				BeforeEach(func() {
					hcsClient.GetLayerMountPathReturns("", nil)
				})

				It("errors", func() {
					_, err := sandboxManager.Create(rootfs, 666)
					Expect(err).To(Equal(&hcsclient.MissingVolumePathError{Id: containerId}))
				})
			})
		})
	})

	Context("Delete", func() {
		BeforeEach(func() {
			hcsClient.LayerExistsReturns(true, nil)
			logrus.SetOutput(ioutil.Discard)
		})

		It("unprepares, deactivates, and destroys the sandbox", func() {
			err := sandboxManager.Delete()
			Expect(err).ToNot(HaveOccurred())

			Expect(hcsClient.LayerExistsCallCount()).To(Equal(1))
			driverInfo, actualContainerId := hcsClient.LayerExistsArgsForCall(0)
			Expect(driverInfo).To(Equal(expectedDriverInfo))
			Expect(actualContainerId).To(Equal(containerId))

			Expect(hcsClient.UnprepareLayerCallCount()).To(Equal(1))
			driverInfo, actualContainerId = hcsClient.UnprepareLayerArgsForCall(0)
			Expect(driverInfo).To(Equal(expectedDriverInfo))
			Expect(actualContainerId).To(Equal(containerId))

			Expect(hcsClient.DeactivateLayerCallCount()).To(Equal(1))
			driverInfo, actualContainerId = hcsClient.DeactivateLayerArgsForCall(0)
			Expect(driverInfo).To(Equal(expectedDriverInfo))
			Expect(actualContainerId).To(Equal(containerId))

			Expect(hcsClient.DestroyLayerCallCount()).To(Equal(1))
			driverInfo, actualContainerId = hcsClient.DestroyLayerArgsForCall(0)
			Expect(driverInfo).To(Equal(expectedDriverInfo))
			Expect(actualContainerId).To(Equal(containerId))
		})

		Context("when checking if the layer exists fails", func() {
			var layerExistsError = errors.New("layer exists failed")

			BeforeEach(func() {
				hcsClient.LayerExistsReturns(false, layerExistsError)
			})

			It("errors", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(layerExistsError))
			})
		})

		Context("when unpreparing the sandbox fails", func() {
			var unprepareLayerError = errors.New("unprepare sandbox failed")

			BeforeEach(func() {
				hcsClient.UnprepareLayerReturns(unprepareLayerError)
			})

			It("errors", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(unprepareLayerError))
			})
		})

		Context("when deactivating the sandbox fails", func() {
			var deactivateLayerError = errors.New("deactivate sandbox failed")

			BeforeEach(func() {
				hcsClient.DeactivateLayerReturns(deactivateLayerError)
			})

			It("errors", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(deactivateLayerError))
			})
		})

		Context("when destroying the sandbox fails", func() {
			var destroyLayerError = errors.New("destroy sandbox failed")

			BeforeEach(func() {
				hcsClient.DestroyLayerReturns(destroyLayerError)
			})

			It("errors", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(destroyLayerError))
			})
		})

		Context("when the sandbox layer does not exist", func() {
			BeforeEach(func() {
				hcsClient.LayerExistsReturns(false, nil)
			})

			It("returns nil and does not try to delete the layer", func() {
				Expect(sandboxManager.Delete()).To(Succeed())
				Expect(hcsClient.LayerExistsCallCount()).To(Equal(1))
				Expect(hcsClient.UnprepareLayerCallCount()).To(Equal(0))
				Expect(hcsClient.DeactivateLayerCallCount()).To(Equal(0))
				Expect(hcsClient.DestroyLayerCallCount()).To(Equal(0))
			})
		})
	})
})
