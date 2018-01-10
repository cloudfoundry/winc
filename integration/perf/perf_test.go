package perf_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"code.cloudfoundry.org/winc/network"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Perf", func() {
	var (
		tempDir           string
		imageStore        string
		bundleDepot       string
		networkConfig     network.Config
		networkConfigFile string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "winc-perf")
		Expect(err).NotTo(HaveOccurred())

		imageStore = filepath.Join(tempDir, "image-store")
		Expect(os.MkdirAll(imageStore, 0666)).To(Succeed())

		bundleDepot = filepath.Join(tempDir, "bundle-depot")
		Expect(os.MkdirAll(bundleDepot, 0666)).To(Succeed())

		networkConfigFile = filepath.Join(tempDir, "winc-perf-network-config.json")
		networkConfig = helpers.GenerateNetworkConfig()
		helpers.CreateNetwork(networkConfig, networkConfigFile)
	})

	AfterEach(func() {
		helpers.DeleteNetwork(networkConfig, networkConfigFile)

		Expect(os.Remove(imageStore)).To(Succeed(), "failed to clean up sandbox image store")
		Expect(os.RemoveAll(tempDir)).To(Succeed())
	})

	It("image, runtime, and network plugins are performant", func() {
		By(fmt.Sprintf("creating, running, and deleting %d sandboxes, containers, and network endpoints concurrently", concurrentContainers), func() {
			var wg sync.WaitGroup
			for i := 0; i < concurrentContainers; i++ {
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()

					containerId := "perf-" + strconv.Itoa(rand.Int())

					bundleSpec := helpers.CreateSandbox(imageStore, rootfsPath, containerId)
					bundleSpec.Process = &specs.Process{Cwd: "C:\\", Args: []string{"cmd.exe"}}
					helpers.CreateContainerWithImageStore(bundleSpec, filepath.Join(bundleDepot, containerId), containerId, imageStore)
					helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {}}`, networkConfigFile)

					containerRun(containerId, "whoami")
					containerRun(containerId, "ipconfig")

					helpers.NetworkDown(containerId, networkConfigFile)
					helpers.DeleteContainerWithImageStore(containerId, imageStore)
					helpers.DeleteSandbox(imageStore, containerId)
				}()
			}
			wg.Wait()
		})
	})
})

func containerRun(containerId string, command ...string) {
	_, _, err := helpers.ExecInContainer(containerId, command, false)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}
