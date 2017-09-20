package image_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/image"
	"code.cloudfoundry.org/winc/image/imagefakes"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create", func() {
	const containerVolume = "containerVolume"
	const containerId = "some-container-id"

	var (
		rootfs        string
		layerManager  *imagefakes.FakeLayerManager
		limiter       *imagefakes.FakeLimiter
		statser       *imagefakes.FakeStatser
		imageManager  *image.Manager
		rootfsParents []byte
		storePath     string
	)

	BeforeEach(func() {
		var err error
		rootfs, err = ioutil.TempDir("", "rootfs")
		Expect(err).ToNot(HaveOccurred())

		storePath, err = ioutil.TempDir("", "store")
		Expect(err).ToNot(HaveOccurred())

		layerManager = &imagefakes.FakeLayerManager{}
		layerManager.HomeDirReturns(storePath)
		limiter = &imagefakes.FakeLimiter{}
		statser = &imagefakes.FakeStatser{}
		imageManager = image.NewManager(layerManager, limiter, statser, containerId)

		rootfsParents = []byte(`["path1", "path2"]`)
	})

	JustBeforeEach(func() {
		Expect(ioutil.WriteFile(filepath.Join(rootfs, "layerchain.json"), rootfsParents, 0755)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(rootfs)).To(Succeed())
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Context("Create", func() {
		Context("when provided a rootfs layer", func() {
			BeforeEach(func() {
				layerManager.CreateLayerStub = func(containerId string, _ string, _ []string) (string, error) {
					Expect(os.MkdirAll(filepath.Join(layerManager.HomeDir(), containerId), 0755)).To(Succeed())
					return containerVolume, nil
				}

				statser.GetCurrentDiskUsageReturns(30000000, nil)
			})

			It("creates and activates the sandbox", func() {
				expectedLayerFolders := []string{rootfs, "path1", "path2"}

				actualImageSpec, err := imageManager.Create(rootfs, 666)
				Expect(err).ToNot(HaveOccurred())
				expectedImageSpec := &image.ImageSpec{
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

				Expect(layerManager.LayerExistsCallCount()).To(Equal(1))
				actualContainerId := layerManager.LayerExistsArgsForCall(0)
				Expect(actualContainerId).To(Equal(containerId))

				Expect(layerManager.CreateLayerCallCount()).To(Equal(1))
				actualContainerId, parentLayer, parentLayers := layerManager.CreateLayerArgsForCall(0)
				Expect(actualContainerId).To(Equal(containerId))
				Expect(parentLayer).To(Equal(rootfs))
				Expect(parentLayers).To(Equal(expectedLayerFolders))

				Expect(limiter.SetDiskLimitCallCount()).To(Equal(1))
				actualContainerVolume, actualDiskLimit := limiter.SetDiskLimitArgsForCall(0)
				Expect(actualContainerVolume).To(Equal(containerVolume))
				Expect(actualDiskLimit).To(BeEquivalentTo(666))
			})

			It("writes the image size to <home-dir>/<id>/image_info", func() {
				_, err := imageManager.Create(rootfs, 666)
				Expect(err).ToNot(HaveOccurred())

				content, err := ioutil.ReadFile(filepath.Join(storePath, containerId, "image_info"))
				Expect(err).ToNot(HaveOccurred())

				Expect(string(content)).To(Equal("30000000"))
			})

			Context("when the layer already exists", func() {
				BeforeEach(func() {
					layerManager.LayerExistsReturns(true, nil)
				})

				It("errors", func() {
					_, err := imageManager.Create(rootfs, 666)
					Expect(err).To(Equal(&image.LayerExistsError{Id: containerId}))
				})
			})

			Context("when creating the sandbox fails with a non-retryable error", func() {
				var createSandboxLayerError = errors.New("create sandbox failed (non-retryable)")

				BeforeEach(func() {
					layerManager.LayerExistsReturnsOnCall(0, false, nil)
					layerManager.LayerExistsReturnsOnCall(1, true, nil)
					layerManager.CreateLayerReturns("", createSandboxLayerError)
					layerManager.RetryableReturns(false)
				})

				It("immediately deletes the sandbox and errors", func() {
					_, err := imageManager.Create(rootfs, 666)
					Expect(err).To(Equal(createSandboxLayerError))

					Expect(layerManager.CreateLayerCallCount()).To(Equal(1))

					Expect(layerManager.LayerExistsCallCount()).To(BeNumerically(">", 0))
					actualContainerId := layerManager.LayerExistsArgsForCall(0)
					Expect(actualContainerId).To(Equal(containerId))

					Expect(layerManager.RemoveLayerCallCount()).To(Equal(1))
					actualContainerId = layerManager.RemoveLayerArgsForCall(0)
					Expect(actualContainerId).To(Equal(containerId))
				})
			})

			Context("when creating the sandbox fails with a retryable error", func() {
				var createLayerError = errors.New("create sandbox failed (retryable)")

				BeforeEach(func() {
					layerManager.LayerExistsReturnsOnCall(0, false, nil)
					layerManager.LayerExistsReturnsOnCall(1, true, nil)
					layerManager.CreateLayerReturns("", createLayerError)
					layerManager.RetryableReturns(true)
				})

				It("tries to create the sandbox CREATE_ATTEMPTS times before returning an error", func() {
					_, err := imageManager.Create(rootfs, 666)
					Expect(err).To(Equal(createLayerError))

					Expect(layerManager.CreateLayerCallCount()).To(Equal(image.CREATE_ATTEMPTS))
					Expect(layerManager.RetryableCallCount()).To(Equal(image.CREATE_ATTEMPTS))
					for i := 0; i < image.CREATE_ATTEMPTS; i++ {
						Expect(layerManager.RetryableArgsForCall(i)).To(Equal(createLayerError))
					}

					Expect(layerManager.LayerExistsCallCount()).To(BeNumerically(">", 0))
					actualContainerId := layerManager.LayerExistsArgsForCall(0)
					Expect(actualContainerId).To(Equal(containerId))

					Expect(layerManager.RemoveLayerCallCount()).To(Equal(1))
					actualContainerId = layerManager.RemoveLayerArgsForCall(0)
					Expect(actualContainerId).To(Equal(containerId))
				})
			})

			Context("when creating the sandbox fails with a retryable error and eventually succeeds", func() {
				var (
					createLayerError = errors.New("create sandbox failed (retryable)")
				)

				BeforeEach(func() {
					attempts := 0
					layerManager.LayerExistsReturnsOnCall(0, false, nil)
					layerManager.CreateLayerStub = func(containerId string, _ string, _ []string) (string, error) {
						attempts += 1
						if attempts < image.CREATE_ATTEMPTS {
							return "", createLayerError
						}
						Expect(os.MkdirAll(filepath.Join(layerManager.HomeDir(), containerId), 0755)).To(Succeed())
						return containerVolume, nil
					}
					layerManager.RetryableReturns(true)
				})

				It("tries to create the sandbox CREATE_ATTEMPTS times", func() {
					actualImageSpec, err := imageManager.Create(rootfs, 666)
					Expect(err).ToNot(HaveOccurred())
					expectedImageSpec := &image.ImageSpec{
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

					Expect(layerManager.CreateLayerCallCount()).To(Equal(image.CREATE_ATTEMPTS))
					Expect(layerManager.RetryableCallCount()).To(Equal(image.CREATE_ATTEMPTS - 1))

					for i := 0; i < image.CREATE_ATTEMPTS-1; i++ {
						Expect(layerManager.RetryableArgsForCall(i)).To(Equal(createLayerError))
					}
				})
			})

			Context("when setting the disk limit fails", func() {
				var diskLimitError = errors.New("setting disk limit failed")

				BeforeEach(func() {
					layerManager.LayerExistsReturnsOnCall(0, false, nil)
					layerManager.LayerExistsReturnsOnCall(1, true, nil)
					limiter.SetDiskLimitReturns(diskLimitError)
				})

				It("deletes the sandbox and errors", func() {
					_, err := imageManager.Create(rootfs, 666)
					Expect(err).To(Equal(diskLimitError))

					Expect(layerManager.LayerExistsCallCount()).To(BeNumerically(">", 0))
					actualContainerId := layerManager.LayerExistsArgsForCall(0)
					Expect(actualContainerId).To(Equal(containerId))

					Expect(layerManager.RemoveLayerCallCount()).To(Equal(1))
					actualContainerId = layerManager.RemoveLayerArgsForCall(0)
					Expect(actualContainerId).To(Equal(containerId))
				})
			})
		})

		Context("when provided a nonexistent rootfs layer", func() {
			It("errors", func() {
				_, err := imageManager.Create("nonexistentrootfs", 666)
				Expect(err).To(Equal(&image.MissingRootfsError{Msg: "nonexistentrootfs"}))
			})
		})

		Context("when provided a rootfs layer missing a layerchain.json", func() {
			JustBeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(rootfs, "layerchain.json"))).To(Succeed())
			})

			It("errors", func() {
				_, err := imageManager.Create(rootfs, 666)
				Expect(err).To(Equal(&image.MissingRootfsLayerChainError{Msg: rootfs}))
			})
		})

		Context("when the rootfs has a layerchain.json that is invalid JSON", func() {
			BeforeEach(func() {
				rootfsParents = []byte("[")
			})

			It("errors", func() {
				_, err := imageManager.Create(rootfs, 666)
				Expect(err).To(Equal(&image.InvalidRootfsLayerChainError{Msg: rootfs}))
			})
		})
	})
})
