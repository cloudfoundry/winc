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

			Expect(client.CreateLayerCallCount()).To(Equal(1))
			di, id, parentId, pls := client.CreateLayerArgsForCall(0)
			Expect(di.HomeDir).To(Equal(storeDir))
			Expect(di.Flavour).To(Equal(1))
			Expect(id).To(Equal(layerId))
			Expect(parentId).To(Equal(rootfsPath))
			Expect(pls).To(Equal(parentLayers))

			Expect(client.GetLayerMountPathCallCount()).To(Equal(1))
			di, id = client.GetLayerMountPathArgsForCall(0)
			Expect(di.HomeDir).To(Equal(storeDir))
			Expect(di.Flavour).To(Equal(1))
			Expect(id).To(Equal(layerId))
		})

		Context("when create, activate, and prepare fail consistently", func() {
			BeforeEach(func() {
				client.CreateLayerReturns(errors.New("create-err"))
			})

			It("returns an error", func() {
				_, err := m.CreateLayer(layerId, rootfsPath, parentLayers)
				Expect(err).To(MatchError("create-err"))

				Expect(client.CreateLayerCallCount()).To(Equal(1))
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
		It("removes the layer", func() {
			Expect(m.RemoveLayer(layerId)).To(Succeed())

			Expect(client.RemoveLayerCallCount()).To(Equal(1))
			di, id := client.RemoveLayerArgsForCall(0)
			Expect(di.HomeDir).To(Equal(storeDir))
			Expect(di.Flavour).To(Equal(1))
			Expect(id).To(Equal(layerId))
		})

		Context("when destroying fails", func() {
			BeforeEach(func() {
				client.RemoveLayerReturns(errors.New("destroy-err"))
			})

			It("returns an error", func() {
				Expect(m.RemoveLayer(layerId)).To(MatchError("destroy-err"))

				Expect(client.RemoveLayerCallCount()).To(Equal(1))
			})
		})
	})
})
