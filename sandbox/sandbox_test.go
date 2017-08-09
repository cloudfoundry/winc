package sandbox_test

import (
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/winc/sandbox"
	"code.cloudfoundry.org/winc/sandbox/sandboxfakes"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
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
		hcsClient          *sandboxfakes.FakeHCSClient
		limiter            *sandboxfakes.FakeLimiter
		sandboxManager     *sandbox.Manager
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

		hcsClient = &sandboxfakes.FakeHCSClient{}
		limiter = &sandboxfakes.FakeLimiter{}
		sandboxManager = sandbox.NewManager(hcsClient, limiter, storePath, containerId)

		expectedDriverInfo = hcsshim.DriverInfo{
			HomeDir: storePath,
			Flavour: 1,
		}
		rootfsParents = []byte(`["path1", "path2"]`)
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
			BeforeEach(func() {
				hcsClient.CreateLayerReturns(containerVolume, nil)
			})

			It("creates and activates the sandbox", func() {
				expectedLayerFolders := []string{rootfs, "path1", "path2"}

				actualImageSpec, err := sandboxManager.Create(rootfs, 666)
				Expect(err).ToNot(HaveOccurred())
				expectedImageSpec := &sandbox.ImageSpec{
					RootFs: containerVolume,
					Spec: specs.Spec{
						Root: &specs.Root{
							Path: containerVolume,
						},
						Windows: &specs.Windows{
							LayerFolders: expectedLayerFolders,
						},
					},
				}
				Expect(actualImageSpec).To(Equal(expectedImageSpec))

				Expect(hcsClient.LayerExistsCallCount()).To(Equal(1))
				driverInfo, actualContainerId := hcsClient.LayerExistsArgsForCall(0)
				Expect(driverInfo).To(Equal(expectedDriverInfo))
				Expect(actualContainerId).To(Equal(containerId))

				Expect(hcsClient.CreateLayerCallCount()).To(Equal(1))
				driverInfo, actualContainerId, parentLayer, parentLayers := hcsClient.CreateLayerArgsForCall(0)
				Expect(driverInfo).To(Equal(expectedDriverInfo))
				Expect(actualContainerId).To(Equal(containerId))
				Expect(parentLayer).To(Equal(rootfs))
				Expect(parentLayers).To(Equal(expectedLayerFolders))

				Expect(limiter.SetDiskLimitCallCount()).To(Equal(1))
				actualContainerVolume, actualDiskLimit := limiter.SetDiskLimitArgsForCall(0)
				Expect(actualContainerVolume).To(Equal(containerVolume))
				Expect(actualDiskLimit).To(BeEquivalentTo(666))
			})

			Context("when the layer already exists", func() {
				BeforeEach(func() {
					hcsClient.LayerExistsReturns(true, nil)
				})

				It("errors", func() {
					_, err := sandboxManager.Create(rootfs, 666)
					Expect(err).To(Equal(&sandbox.LayerExistsError{Id: containerId}))
				})
			})

			Context("when creating the sandbox fails with a non-retryable error", func() {
				var createSandboxLayerError = errors.New("create sandbox failed (non-retryable)")

				BeforeEach(func() {
					hcsClient.LayerExistsReturnsOnCall(0, false, nil)
					hcsClient.LayerExistsReturnsOnCall(1, true, nil)
					hcsClient.CreateLayerReturns("", createSandboxLayerError)
					hcsClient.RetryableReturns(false)
				})

				It("immediately deletes the sandbox and errors", func() {
					_, err := sandboxManager.Create(rootfs, 666)
					Expect(err).To(Equal(createSandboxLayerError))

					Expect(hcsClient.CreateLayerCallCount()).To(Equal(1))

					Expect(hcsClient.LayerExistsCallCount()).To(BeNumerically(">", 0))
					driverInfo, actualContainerId := hcsClient.LayerExistsArgsForCall(0)
					Expect(driverInfo).To(Equal(expectedDriverInfo))
					Expect(actualContainerId).To(Equal(containerId))

					Expect(hcsClient.DestroyLayerCallCount()).To(Equal(1))
					driverInfo, actualContainerId = hcsClient.DestroyLayerArgsForCall(0)
					Expect(driverInfo).To(Equal(expectedDriverInfo))
					Expect(actualContainerId).To(Equal(containerId))
				})
			})

			Context("when creating the sandbox fails with a retryable error", func() {
				var createLayerError = errors.New("create sandbox failed (retryable)")

				BeforeEach(func() {
					hcsClient.LayerExistsReturnsOnCall(0, false, nil)
					hcsClient.LayerExistsReturnsOnCall(1, true, nil)
					hcsClient.CreateLayerReturns("", createLayerError)
					hcsClient.RetryableReturns(true)
				})

				It("tries to create the sandbox CREATE_ATTEMPTS times before returning an error", func() {
					_, err := sandboxManager.Create(rootfs, 666)
					Expect(err).To(Equal(createLayerError))

					Expect(hcsClient.CreateLayerCallCount()).To(Equal(sandbox.CREATE_ATTEMPTS))
					Expect(hcsClient.RetryableCallCount()).To(Equal(sandbox.CREATE_ATTEMPTS))
					for i := 0; i < sandbox.CREATE_ATTEMPTS; i++ {
						Expect(hcsClient.RetryableArgsForCall(i)).To(Equal(createLayerError))
					}

					Expect(hcsClient.LayerExistsCallCount()).To(BeNumerically(">", 0))
					driverInfo, actualContainerId := hcsClient.LayerExistsArgsForCall(0)
					Expect(driverInfo).To(Equal(expectedDriverInfo))
					Expect(actualContainerId).To(Equal(containerId))

					Expect(hcsClient.DestroyLayerCallCount()).To(Equal(1))
					driverInfo, actualContainerId = hcsClient.DestroyLayerArgsForCall(0)
					Expect(driverInfo).To(Equal(expectedDriverInfo))
					Expect(actualContainerId).To(Equal(containerId))
				})
			})

			Context("when creating the sandbox fails with a retryable error and eventually succeeds", func() {
				var createLayerError = errors.New("create sandbox failed (retryable)")

				BeforeEach(func() {
					hcsClient.LayerExistsReturnsOnCall(0, false, nil)
					hcsClient.CreateLayerReturnsOnCall(0, "", createLayerError)
					hcsClient.CreateLayerReturnsOnCall(1, "", createLayerError)
					hcsClient.CreateLayerReturnsOnCall(2, containerVolume, nil)
					hcsClient.RetryableReturns(true)
				})

				It("tries to create the sandbox three times", func() {
					actualImageSpec, err := sandboxManager.Create(rootfs, 666)
					Expect(err).ToNot(HaveOccurred())
					expectedImageSpec := &sandbox.ImageSpec{
						RootFs: containerVolume,
						Spec: specs.Spec{
							Root: &specs.Root{
								Path: containerVolume,
							},
							Windows: &specs.Windows{
								LayerFolders: []string{rootfs, "path1", "path2"},
							},
						},
					}
					Expect(actualImageSpec).To(Equal(expectedImageSpec))

					Expect(hcsClient.CreateLayerCallCount()).To(Equal(3))
					Expect(hcsClient.RetryableCallCount()).To(Equal(2))
					Expect(hcsClient.RetryableArgsForCall(0)).To(Equal(createLayerError))
					Expect(hcsClient.RetryableArgsForCall(1)).To(Equal(createLayerError))
				})
			})

			Context("when setting the disk limit fails", func() {
				var diskLimitError = errors.New("setting disk limit failed")

				BeforeEach(func() {
					hcsClient.LayerExistsReturnsOnCall(0, false, nil)
					hcsClient.LayerExistsReturnsOnCall(1, true, nil)
					limiter.SetDiskLimitReturns(diskLimitError)
				})

				It("deletes the sandbox and errors", func() {
					_, err := sandboxManager.Create(rootfs, 666)
					Expect(err).To(Equal(diskLimitError))

					Expect(hcsClient.LayerExistsCallCount()).To(BeNumerically(">", 0))
					driverInfo, actualContainerId := hcsClient.LayerExistsArgsForCall(0)
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

		Context("when the sandbox layer does not exist", func() {
			BeforeEach(func() {
				hcsClient.LayerExistsReturns(false, nil)
			})

			It("returns nil and does not try to delete the layer", func() {
				Expect(sandboxManager.Delete()).To(Succeed())
				Expect(hcsClient.LayerExistsCallCount()).To(Equal(1))
				Expect(hcsClient.DestroyLayerCallCount()).To(Equal(0))
			})
		})

		Context("when destroying the sandbox fails with a non-retryable error", func() {
			var destroyLayerError = errors.New("destroy sandbox failed (non-retryable)")

			BeforeEach(func() {
				hcsClient.DestroyLayerReturns(destroyLayerError)
				hcsClient.RetryableReturns(false)
			})

			It("errors immediately", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(destroyLayerError))

				Expect(hcsClient.DestroyLayerCallCount()).To(Equal(1))
				Expect(hcsClient.RetryableCallCount()).To(Equal(1))
				Expect(hcsClient.RetryableArgsForCall(0)).To(Equal(destroyLayerError))
			})
		})

		Context("when destroying the sandbox fails with a retryable error", func() {
			var destroyLayerError = errors.New("destroy sandbox failed (retryable)")

			BeforeEach(func() {
				hcsClient.DestroyLayerReturns(destroyLayerError)
				hcsClient.RetryableReturns(true)
			})

			It("tries to destroy the sandbox DESTROY_ATTEMPTS times before returning an error", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(destroyLayerError))

				Expect(hcsClient.DestroyLayerCallCount()).To(Equal(sandbox.DESTROY_ATTEMPTS))
				Expect(hcsClient.RetryableCallCount()).To(Equal(sandbox.DESTROY_ATTEMPTS))
				for i := 0; i < sandbox.DESTROY_ATTEMPTS; i++ {
					Expect(hcsClient.RetryableArgsForCall(i)).To(Equal(destroyLayerError))
				}
			})
		})

		Context("when destroying the sandbox fails with a retryable error and eventually succeeds", func() {
			var destroyLayerError = errors.New("destroy sandbox failed (retryable)")

			BeforeEach(func() {
				hcsClient.DestroyLayerReturnsOnCall(0, destroyLayerError)
				hcsClient.DestroyLayerReturnsOnCall(1, destroyLayerError)
				hcsClient.DestroyLayerReturnsOnCall(2, nil)
				hcsClient.RetryableReturns(true)
			})

			It("tries to destroy the sandbox three times", func() {
				Expect(sandboxManager.Delete()).To(Succeed())

				Expect(hcsClient.DestroyLayerCallCount()).To(Equal(3))
				Expect(hcsClient.RetryableCallCount()).To(Equal(2))
				Expect(hcsClient.RetryableArgsForCall(0)).To(Equal(destroyLayerError))
				Expect(hcsClient.RetryableArgsForCall(1)).To(Equal(destroyLayerError))
			})
		})
	})
})
