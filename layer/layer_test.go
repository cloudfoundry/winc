package layer_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/layer"
	"code.cloudfoundry.org/winc/layer/layerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {
	var (
		client       *layerfakes.FakeHCSClient
		m            *layer.Manager
		parentLayers []string
		storeDir     string
		rootfsPath   string
	)

	const expectedVolumeGuid = `\\?\Volume{some-guid}\`
	const layerId = "some-layer-id"

	BeforeEach(func() {
		tmpDir, err := ioutil.TempDir("", "store-layer-test")
		Expect(err).NotTo(HaveOccurred())

		storeDir = filepath.Join(tmpDir, "layer-home-dir")

		rootfsPath = "rootfs"
		parentLayers = []string{"rootfs", "layer-2", "layer-1"}

		client = &layerfakes.FakeHCSClient{}
		m = layer.NewManager(client, storeDir)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storeDir)).To(Succeed())
	})

	Describe("CreateLayer", func() {
		BeforeEach(func() {
			client.GetLayerMountPathReturns(expectedVolumeGuid, nil)
		})

		It("creates the layer", func() {
			volumeGuid, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
			Expect(err).ToNot(HaveOccurred())
			Expect(volumeGuid).To(Equal(expectedVolumeGuid))

			Expect(storeDir).To(BeADirectory())

			Expect(client.CreateSandboxLayerCallCount()).To(Equal(1))
			di, id, parentId, pls := client.CreateSandboxLayerArgsForCall(0)
			Expect(di.HomeDir).To(Equal(storeDir))
			Expect(di.Flavour).To(Equal(1))
			Expect(id).To(Equal(layerId))
			Expect(parentId).To(Equal(rootfsPath))
			Expect(pls).To(Equal(parentLayers))

			Expect(client.ActivateLayerCallCount()).To(Equal(1))
			di, id = client.ActivateLayerArgsForCall(0)
			Expect(di.HomeDir).To(Equal(storeDir))
			Expect(di.Flavour).To(Equal(1))
			Expect(id).To(Equal(layerId))

			Expect(client.PrepareLayerCallCount()).To(Equal(1))
			di, id, pls = client.PrepareLayerArgsForCall(0)
			Expect(di.HomeDir).To(Equal(storeDir))
			Expect(di.Flavour).To(Equal(1))
			Expect(id).To(Equal(layerId))
			Expect(pls).To(Equal(parentLayers))

			Expect(client.GetLayerMountPathCallCount()).To(Equal(1))
			di, id = client.GetLayerMountPathArgsForCall(0)
			Expect(di.HomeDir).To(Equal(storeDir))
			Expect(di.Flavour).To(Equal(1))
			Expect(id).To(Equal(layerId))
		})

		Context("when CreateSandboxLayer times out and CreateLayer() is retried", func() {
			BeforeEach(func() {
				client.CreateSandboxLayerReturnsOnCall(0, errors.New("HCS timed out"))
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err.Error()).To(Equal("HCS timed out"))
			})

			It("calls all of the layer creation HCS functions", func() {
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err).ToNot(HaveOccurred())

				Expect(client.CreateSandboxLayerCallCount()).To(Equal(2))
				Expect(client.ActivateLayerCallCount()).To(Equal(1))
				Expect(client.PrepareLayerCallCount()).To(Equal(1))
				Expect(client.GetLayerMountPathCallCount()).To(Equal(1))
			})
		})

		Context("when the sandbox layer is created, ActivateLayer times out and CreateLayer() is retried", func() {
			BeforeEach(func() {
				client.ActivateLayerReturnsOnCall(0, errors.New("HCS timed out"))
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err.Error()).To(Equal("HCS timed out"))
			})

			It("starts the layer creation process from ActivateLayer", func() {
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err).ToNot(HaveOccurred())

				Expect(client.CreateSandboxLayerCallCount()).To(Equal(1))
				Expect(client.ActivateLayerCallCount()).To(Equal(2))
				Expect(client.PrepareLayerCallCount()).To(Equal(1))
				Expect(client.GetLayerMountPathCallCount()).To(Equal(1))
			})
		})

		Context("when the sandbox layer is created + activated, PrepareLayer times out and CreateLayer() is retried", func() {
			BeforeEach(func() {
				client.PrepareLayerReturnsOnCall(0, errors.New("HCS timed out"))
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err.Error()).To(Equal("HCS timed out"))
			})

			It("starts the layer creation process from PrepareLayer", func() {
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err).ToNot(HaveOccurred())

				Expect(client.CreateSandboxLayerCallCount()).To(Equal(1))
				Expect(client.ActivateLayerCallCount()).To(Equal(1))
				Expect(client.PrepareLayerCallCount()).To(Equal(2))
				Expect(client.GetLayerMountPathCallCount()).To(Equal(1))
			})
		})

		Context("when the sandbox layer is created + activated + prepared, GetLayerMountPath times out and CreateLayer() is retried", func() {
			BeforeEach(func() {
				client.GetLayerMountPathReturnsOnCall(0, "", errors.New("HCS timed out"))
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err.Error()).To(Equal("HCS timed out"))
			})

			It("only retries GetLayerMountPath", func() {
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err).ToNot(HaveOccurred())

				Expect(client.CreateSandboxLayerCallCount()).To(Equal(1))
				Expect(client.ActivateLayerCallCount()).To(Equal(1))
				Expect(client.PrepareLayerCallCount()).To(Equal(1))
				Expect(client.GetLayerMountPathCallCount()).To(Equal(2))
			})
		})

		Context("GetLayerMountPath returns an empty string", func() {
			BeforeEach(func() {
				client.GetLayerMountPathReturnsOnCall(0, "", nil)
			})

			It("returns a missing volume path error", func() {
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err).To(MatchError(&layer.MissingVolumePathError{Id: layerId}))
			})
		})
	})

	Describe("RemoveLayer", func() {
		It("destroys the layer", func() {
			Expect(m.RemoveLayer(layerId)).To(Succeed())

			Expect(client.UnprepareLayerCallCount()).To(Equal(1))
			di, id := client.UnprepareLayerArgsForCall(0)
			Expect(di.HomeDir).To(Equal(storeDir))
			Expect(di.Flavour).To(Equal(1))
			Expect(id).To(Equal(layerId))

			Expect(client.DeactivateLayerCallCount()).To(Equal(1))
			di, id = client.DeactivateLayerArgsForCall(0)
			Expect(di.HomeDir).To(Equal(storeDir))
			Expect(di.Flavour).To(Equal(1))
			Expect(id).To(Equal(layerId))

			Expect(client.DestroyLayerCallCount()).To(Equal(1))
			di, id = client.DestroyLayerArgsForCall(0)
			Expect(di.HomeDir).To(Equal(storeDir))
			Expect(di.Flavour).To(Equal(1))
			Expect(id).To(Equal(layerId))
		})

		Context("when UnprepareLayer times out and RemoveLayer is retried", func() {
			BeforeEach(func() {
				client.UnprepareLayerReturnsOnCall(0, errors.New("HCS timed out"))
				err := m.RemoveLayer(layerId)
				Expect(err.Error()).To(Equal("HCS timed out"))
			})

			It("calls all of the layer deletion HCS functions", func() {
				Expect(m.RemoveLayer(layerId)).To(Succeed())

				Expect(client.UnprepareLayerCallCount()).To(Equal(2))
				Expect(client.DeactivateLayerCallCount()).To(Equal(1))
				Expect(client.DestroyLayerCallCount()).To(Equal(1))
			})
		})

		Context("when the layer has been unprepared, DeactivateLayer times out, and RemoveLayer is retried", func() {
			BeforeEach(func() {
				client.DeactivateLayerReturnsOnCall(0, errors.New("HCS timed out"))
				err := m.RemoveLayer(layerId)
				Expect(err.Error()).To(Equal("HCS timed out"))
			})

			It("starts the layer deletion process from DeactivateLayer", func() {
				Expect(m.RemoveLayer(layerId)).To(Succeed())

				Expect(client.UnprepareLayerCallCount()).To(Equal(1))
				Expect(client.DeactivateLayerCallCount()).To(Equal(2))
				Expect(client.DestroyLayerCallCount()).To(Equal(1))
			})
		})

		Context("when the layer has been unprepared + deactivated, DestroyLayer times out, and RemoveLayer is retried", func() {
			BeforeEach(func() {
				client.DestroyLayerReturnsOnCall(0, errors.New("HCS timed out"))
				err := m.RemoveLayer(layerId)
				Expect(err.Error()).To(Equal("HCS timed out"))
			})

			It("only calls DestroyLayer", func() {
				Expect(m.RemoveLayer(layerId)).To(Succeed())

				Expect(client.UnprepareLayerCallCount()).To(Equal(1))
				Expect(client.DeactivateLayerCallCount()).To(Equal(1))
				Expect(client.DestroyLayerCallCount()).To(Equal(2))
			})
		})
	})

	Describe("Retryable", func() {
		Context("when the error is a timeout error", func() {
			It("returns true", func() {
				err := errors.New("Some operation failed: This operation returned because the timeout period expired")
				Expect(m.Retryable(err)).To(BeTrue())
			})
		})

		Context("when the error is something else", func() {
			It("returns false", func() {
				err := errors.New("some other error")
				Expect(m.Retryable(err)).To(BeFalse())
			})
		})
	})
})
