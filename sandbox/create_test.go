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
		statser            *sandboxfakes.FakeStatser
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
		statser = &sandboxfakes.FakeStatser{}
		sandboxManager = sandbox.NewManager(hcsClient, limiter, statser, storePath, containerId)

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
				hcsClient.CreateLayerStub = func(driverInfo hcsshim.DriverInfo, containerId string, _ string, _ []string) (string, error) {
					Expect(os.MkdirAll(filepath.Join(driverInfo.HomeDir, containerId), 0755)).To(Succeed())
					return containerVolume, nil
				}

				statser.GetCurrentDiskUsageReturns(30000000, nil)
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

			It("writes the image size to <home-dir>/<id>/image_info", func() {
				_, err := sandboxManager.Create(rootfs, 666)
				Expect(err).ToNot(HaveOccurred())

				content, err := ioutil.ReadFile(filepath.Join(storePath, containerId, "image_info"))
				Expect(err).ToNot(HaveOccurred())

				Expect(string(content)).To(Equal("30000000"))
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
				var (
					createLayerError = errors.New("create sandbox failed (retryable)")
				)

				BeforeEach(func() {
					attempts := 0
					hcsClient.LayerExistsReturnsOnCall(0, false, nil)
					hcsClient.CreateLayerStub = func(driverInfo hcsshim.DriverInfo, containerId string, _ string, _ []string) (string, error) {
						attempts += 1
						if attempts < 3 {
							return "", createLayerError
						}
						Expect(os.MkdirAll(filepath.Join(driverInfo.HomeDir, containerId), 0755)).To(Succeed())
						return containerVolume, nil
					}
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
})
