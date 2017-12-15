package perf_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"code.cloudfoundry.org/winc/network"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Perf", func() {
	var (
		tempDir           string
		imageStore        string
		bundleDepot       string
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

		createNetwork(networkConfigFile)
	})

	AfterEach(func() {
		deleteNetwork(networkConfigFile)

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

					bundleSpec := createSandbox(imageStore, rootfsPath, containerId)
					createContainer(imageStore, bundleDepot, bundleSpec, containerId)
					networkUp(networkConfigFile, containerId)

					containerRun(containerId, "whoami")
					containerRun(containerId, "ipconfig")

					networkDown(networkConfigFile, containerId)
					deleteContainer(imageStore, bundleDepot, containerId)
					deleteSandbox(imageStore, containerId)
				}()
			}
			wg.Wait()
		})
	})
})

func execute(cmd *exec.Cmd) (*bytes.Buffer, *bytes.Buffer, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	if cmd.Stdout == nil {
		cmd.Stdout = io.MultiWriter(stdOut, GinkgoWriter)
	}

	if cmd.Stderr == nil {
		cmd.Stderr = io.MultiWriter(stdErr, GinkgoWriter)

	}

	err := cmd.Run()
	return stdOut, stdErr, err
}

func containerRun(containerId string, command ...string) {
	_, _, err := execute(exec.Command(wincBin, append([]string{"exec", containerId}, command...)...))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func createSandbox(storePath, rootfsPath, containerId string) specs.Spec {
	stdOut, _, err := execute(exec.Command(wincImageBin, "--store", storePath, "create", rootfsPath, containerId))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	var spec specs.Spec
	ExpectWithOffset(1, json.Unmarshal(stdOut.Bytes(), &spec)).To(Succeed())
	spec.Process = &specs.Process{
		Args: []string{"cmd"},
		Cwd:  "C:\\",
	}
	return spec
}

func deleteSandbox(storePath, containerId string) {
	_, _, err := execute(exec.Command(wincImageBin, "--store", storePath, "delete", containerId))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func createContainer(imageStore, bundleDepot string, bundleSpec specs.Spec, containerId string) {
	bundlePath := filepath.Join(bundleDepot, containerId)
	ExpectWithOffset(1, os.MkdirAll(bundlePath, 0666)).To(Succeed())

	containerConfig, err := json.Marshal(&bundleSpec)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), containerConfig, 0666)).To(Succeed())

	_, _, err = execute(exec.Command(wincBin, "--image-store", imageStore, "create", "-b", bundlePath, containerId))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func deleteContainer(imageStore, bundleDepot, containerId string) {
	_, _, err := execute(exec.Command(wincBin, "--image-store", imageStore, "delete", containerId))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	ExpectWithOffset(1, os.RemoveAll(filepath.Join(bundleDepot, containerId))).To(Succeed())
}

func networkUp(configFile, containerId string) {
	cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
	cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {}}`)
	_, _, err := execute(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func networkDown(configFile, containerId string) {
	cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "down", "--handle", containerId)
	cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {}}`)
	_, _, err := execute(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func randomSubnetAddress() (string, string) {
	for {
		subnet, gateway := randomValidSubnetAddress()
		_, err := hcsshim.GetHNSNetworkByName(subnet)
		if err != nil {
			ExpectWithOffset(1, err).To(MatchError(ContainSubstring("Network " + subnet + " not found")))
			return subnet, gateway
		}
	}
}

func randomValidSubnetAddress() (string, string) {
	randomOctet := rand.Intn(256)
	gatewayAddress := fmt.Sprintf("172.0.%d.1", randomOctet)
	subnet := fmt.Sprintf("172.0.%d.0/24", randomOctet)
	return subnet, gatewayAddress
}

func createNetwork(configFile string) {
	subnetRange, gatewayAddress := randomSubnetAddress()
	networkConfig := network.Config{
		SubnetRange:    subnetRange,
		GatewayAddress: gatewayAddress,
		NetworkName:    gatewayAddress,
	}
	c, err := json.Marshal(networkConfig)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, ioutil.WriteFile(configFile, c, 0644)).To(Succeed())

	args := []string{"--action", "create", "--configFile", configFile}
	output, err := exec.Command(wincNetworkBin, args...).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func deleteNetwork(configFile string) {
	output, err := exec.Command(wincNetworkBin, "--action", "delete", "--configFile", configFile).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}
