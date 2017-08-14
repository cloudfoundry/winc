package sandbox_test

import (
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/winc/sandbox"
	"code.cloudfoundry.org/winc/sandbox/sandboxfakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stats", func() {
	const containerVolume = "containerVolume"

	var (
		storePath      string
		rootfs         string
		containerId    string
		hcsClient      *sandboxfakes.FakeHCSClient
		limiter        *sandboxfakes.FakeLimiter
		statser        *sandboxfakes.FakeStatser
		sandboxManager *sandbox.Manager
		rootfsParents  []byte
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

		rootfsParents = []byte(`["path1", "path2"]`)
		hcsClient.CreateLayerStub = func(driverInfo hcsshim.DriverInfo, containerId string, _ string, _ []string) (string, error) {
			Expect(os.MkdirAll(filepath.Join(driverInfo.HomeDir, containerId), 0755)).To(Succeed())
			return containerVolume, nil
		}

		statser.GetCurrentDiskUsageReturnsOnCall(0, 30000000, nil)
		statser.GetCurrentDiskUsageReturnsOnCall(1, 30001234, nil)
	})

	JustBeforeEach(func() {
		Expect(ioutil.WriteFile(filepath.Join(rootfs, "layerchain.json"), rootfsParents, 0755)).To(Succeed())
		_, err := sandboxManager.Create(rootfs, 666)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
		Expect(os.RemoveAll(rootfs)).To(Succeed())
	})

	It("returns the stats", func() {
		stats, err := sandboxManager.Stats()
		Expect(err).ToNot(HaveOccurred())
		Expect(stats.Disk.ExclusiveBytesUsed).To(Equal(uint64(1234)))
		Expect(stats.Disk.TotalBytesUsed).To(Equal(uint64(30001234)))
	})
})
