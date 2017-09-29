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

		Context("when create fails but activate and prepare succeed", func() {
			BeforeEach(func() {
				client.CreateSandboxLayerReturnsOnCall(0, errors.New("create-err"))
			})

			It("successfully creates the layer", func() {
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err).ToNot(HaveOccurred())

				Expect(client.CreateSandboxLayerCallCount()).To(Equal(1))
				Expect(client.ActivateLayerCallCount()).To(Equal(1))
				Expect(client.PrepareLayerCallCount()).To(Equal(1))
				Expect(client.GetLayerMountPathCallCount()).To(Equal(1))
			})
		})

		Context("when create and activate fail but prepare succeeds", func() {
			BeforeEach(func() {
				client.CreateSandboxLayerReturnsOnCall(0, errors.New("create-err"))
				client.ActivateLayerReturnsOnCall(0, errors.New("activate-err"))
			})

			It("successfully creates the layer", func() {
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err).ToNot(HaveOccurred())

				Expect(client.CreateSandboxLayerCallCount()).To(Equal(1))
				Expect(client.ActivateLayerCallCount()).To(Equal(1))
				Expect(client.PrepareLayerCallCount()).To(Equal(1))
				Expect(client.GetLayerMountPathCallCount()).To(Equal(1))
			})
		})

		Context("when create, activate, and prepare fail once", func() {
			BeforeEach(func() {
				client.CreateSandboxLayerReturnsOnCall(0, errors.New("create-err"))
				client.ActivateLayerReturnsOnCall(0, errors.New("activate-err"))
				client.PrepareLayerReturnsOnCall(0, errors.New("prepare-err"))
			})

			It("retries and successfully creates the layer", func() {
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err).ToNot(HaveOccurred())

				Expect(client.CreateSandboxLayerCallCount()).To(Equal(2))
				Expect(client.ActivateLayerCallCount()).To(Equal(2))
				Expect(client.PrepareLayerCallCount()).To(Equal(2))
				Expect(client.GetLayerMountPathCallCount()).To(Equal(1))
			})
		})

		Context("when create, activate, and prepare fail consistently", func() {
			BeforeEach(func() {
				client.CreateSandboxLayerReturns(errors.New("create-err"))
				client.ActivateLayerReturns(errors.New("activate-err"))
				client.PrepareLayerReturns(errors.New("prepare-err"))
			})

			It("tries three times and returns an error", func() {
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err).To(MatchError("failed to create layer (create error: create-err, activate error: activate-err, prepare error: prepare-err)"))

				Expect(client.CreateSandboxLayerCallCount()).To(Equal(3))
				Expect(client.ActivateLayerCallCount()).To(Equal(3))
				Expect(client.PrepareLayerCallCount()).To(Equal(3))
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

		Context("when unprepare fails but deactivate and destroy succeed", func() {
			BeforeEach(func() {
				client.UnprepareLayerReturnsOnCall(0, errors.New("unprepare-err"))
			})

			It("successfully destroys the layer", func() {
				Expect(m.RemoveLayer(layerId)).To(Succeed())

				Expect(client.UnprepareLayerCallCount()).To(Equal(1))
				Expect(client.DeactivateLayerCallCount()).To(Equal(1))
				Expect(client.DestroyLayerCallCount()).To(Equal(1))
			})
		})

		Context("when unprepare and deactivate fail but destroy succeeds", func() {
			BeforeEach(func() {
				client.UnprepareLayerReturnsOnCall(0, errors.New("unprepare-err"))
				client.DeactivateLayerReturnsOnCall(0, errors.New("deactivate-err"))
			})

			It("successfully destroys the layer", func() {
				Expect(m.RemoveLayer(layerId)).To(Succeed())

				Expect(client.UnprepareLayerCallCount()).To(Equal(1))
				Expect(client.DeactivateLayerCallCount()).To(Equal(1))
				Expect(client.DestroyLayerCallCount()).To(Equal(1))
			})
		})

		Context("when unprepare, deactivate, and destroy fail once", func() {
			BeforeEach(func() {
				client.UnprepareLayerReturnsOnCall(0, errors.New("unprepare-err"))
				client.DeactivateLayerReturnsOnCall(0, errors.New("deactivate-err"))
				client.DestroyLayerReturnsOnCall(0, errors.New("destroy-err"))
			})

			It("retries and successfully deletes the layer", func() {
				Expect(m.RemoveLayer(layerId)).To(Succeed())

				Expect(client.UnprepareLayerCallCount()).To(Equal(2))
				Expect(client.DeactivateLayerCallCount()).To(Equal(2))
				Expect(client.DestroyLayerCallCount()).To(Equal(2))
			})
		})

		Context("when unprepare, deactivate, and destroy fail consistently", func() {
			BeforeEach(func() {
				client.UnprepareLayerReturns(errors.New("unprepare-err"))
				client.DeactivateLayerReturns(errors.New("deactivate-err"))
				client.DestroyLayerReturns(errors.New("destroy-err"))
			})

			It("tries three times and returns an error", func() {
				Expect(m.RemoveLayer(layerId)).To(MatchError("failed to remove layer (unprepare error: unprepare-err, deactivate error: deactivate-err, destroy error: destroy-err)"))

				Expect(client.UnprepareLayerCallCount()).To(Equal(3))
				Expect(client.DeactivateLayerCallCount()).To(Equal(3))
				Expect(client.DestroyLayerCallCount()).To(Equal(3))
			})
		})
	})
})
