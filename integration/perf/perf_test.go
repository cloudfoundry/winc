package perf_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/winc/network"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sync/errgroup"
)

var _ = Describe("Perf", func() {
	var (
		tempDir           string
		bundleDepot       string
		networkConfig     network.Config
		networkConfigFile string
		containerIds      []string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "winc-perf")
		Expect(err).NotTo(HaveOccurred())

		bundleDepot = filepath.Join(tempDir, "bundle-depot")
		Expect(os.MkdirAll(bundleDepot, 0666)).To(Succeed())

		networkConfigFile = filepath.Join(tempDir, "winc-perf-network-config.json")
		networkConfig = helpers.GenerateNetworkConfig()
		helpers.CreateNetwork(networkConfig, networkConfigFile)
	})

	AfterEach(func() {
		failed = failed || CurrentGinkgoTestDescription().Failed
		for _, containerId := range containerIds {
			helpers.NetworkDown(containerId, networkConfigFile)
			helpers.DeleteContainer(containerId)
			helpers.DeleteVolume(containerId)
		}

		helpers.DeleteNetwork(networkConfig, networkConfigFile)

		Expect(os.RemoveAll(tempDir)).To(Succeed())
	})

	It("image, runtime, and network plugins are performant", func() {
		By(fmt.Sprintf("creating, running, and deleting %d sandboxes, containers, and network endpoints concurrently", concurrentContainers), func() {
			g, _ := errgroup.WithContext(context.Background())

			for i := 0; i < concurrentContainers; i++ {
				containerId := "perf-" + strconv.Itoa(rand.Int())
				g.Go(func() error {
					defer GinkgoRecover()

					bundleSpec := helpers.CreateVolume(rootfsURI, containerId)
					bundleSpec.Process = &specs.Process{Cwd: "C:\\", Args: []string{"cmd.exe"}}
					helpers.CreateContainer(bundleSpec, filepath.Join(bundleDepot, containerId), containerId)
					helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {}}`, networkConfigFile)

					containerRun(containerId, "whoami")
					containerRun(containerId, "ipconfig")

					helpers.NetworkDown(containerId, networkConfigFile)
					helpers.DeleteContainer(containerId)
					helpers.DeleteVolume(containerId)

					return nil
				})
			}

			Expect(g.Wait()).To(Succeed())
		})
	})

	It("reports the correct state", func() {
		By(fmt.Sprintf("creating %d containers concurrently", concurrentContainers), func() {
			g, _ := errgroup.WithContext(context.Background())

			for i := 0; i < concurrentContainers; i++ {
				containerId := "perf-" + strconv.Itoa(rand.Int())
				g.Go(func() error {
					defer GinkgoRecover()

					bundleSpec := helpers.CreateVolume(rootfsURI, containerId)
					bundleSpec.Process = &specs.Process{Cwd: "C:\\", Args: []string{"cmd.exe", "/C", "echo hi"}}
					helpers.CreateContainer(bundleSpec, filepath.Join(bundleDepot, containerId), containerId)

					defer helpers.DeleteVolume(containerId)
					defer helpers.DeleteContainer(containerId)

					helpers.StartContainer(containerId)
					helpers.TheProcessExits(containerId, "cmd.exe")

					state := helpers.GetContainerState(containerId)
					Expect(state.Status).To(Equal("stopped"))

					return nil
				})
			}

			Expect(g.Wait()).To(Succeed())
		})
	})
})

func containerRun(containerId string, command ...string) {
	_, _, err := helpers.ExecInContainer(containerId, command, false)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}
