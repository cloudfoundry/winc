package image_test

import (
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/winc/image"
	"code.cloudfoundry.org/winc/image/imagefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stats", func() {
	const containerVolume = "containerVolume"

	var (
		storePath     string
		rootfs        string
		containerId   string
		layerManager  *imagefakes.FakeLayerManager
		limiter       *imagefakes.FakeLimiter
		statser       *imagefakes.FakeStatser
		imageManager  *image.Manager
		rootfsParents []byte
	)

	BeforeEach(func() {
		var err error
		rootfs, err = ioutil.TempDir("", "rootfs")
		Expect(err).ToNot(HaveOccurred())

		storePath, err = ioutil.TempDir("", "sandbox-store")
		Expect(err).ToNot(HaveOccurred())

		rand.Seed(time.Now().UnixNano())
		containerId = strconv.Itoa(rand.Int())

		layerManager = &imagefakes.FakeLayerManager{}
		layerManager.HomeDirReturns(storePath)
		limiter = &imagefakes.FakeLimiter{}
		statser = &imagefakes.FakeStatser{}
		imageManager = image.NewManager(layerManager, limiter, statser, containerId)

		rootfsParents = []byte(`["path1", "path2"]`)
		layerManager.CreateLayerStub = func(containerId string, _ string, _ []string) (string, error) {
			Expect(os.MkdirAll(filepath.Join(layerManager.HomeDir(), containerId), 0755)).To(Succeed())
			return containerVolume, nil
		}

		statser.GetCurrentDiskUsageReturnsOnCall(0, 30000000, nil)
		statser.GetCurrentDiskUsageReturnsOnCall(1, 30001234, nil)
	})

	JustBeforeEach(func() {
		Expect(ioutil.WriteFile(filepath.Join(rootfs, "layerchain.json"), rootfsParents, 0755)).To(Succeed())
		_, err := imageManager.Create(rootfs, 666)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
		Expect(os.RemoveAll(rootfs)).To(Succeed())
	})

	It("returns the stats", func() {
		stats, err := imageManager.Stats()
		Expect(err).ToNot(HaveOccurred())
		Expect(stats.Disk.ExclusiveBytesUsed).To(Equal(uint64(1234)))
		Expect(stats.Disk.TotalBytesUsed).To(Equal(uint64(30001234)))
	})
})
