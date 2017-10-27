package main_test

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
	"time"

	"code.cloudfoundry.org/winc/image"
	"code.cloudfoundry.org/winc/network"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"testing"
)

const (
	defaultTimeout     = time.Second * 10
	defaultInterval    = time.Millisecond * 200
	maxStandardIPOctet = 256
)

var (
	wincBin           string
	wincNetworkBin    string
	wincImageBin      string
	serverBin         string
	rootfsPath        string
	bundlePath        string
	subnetRange       string
	gatewayAddress    string
	networkConfigFile string
)

func TestWincNetwork(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(defaultTimeout)
	SetDefaultEventuallyPollingInterval(defaultInterval)
	RunSpecs(t, "Winc-Network Suite")
}

var _ = BeforeSuite(func() {
	rand.Seed(time.Now().UnixNano() + int64(GinkgoParallelNode()))

	var (
		present bool
		err     error
	)

	rootfsPath, present = os.LookupEnv("WINC_TEST_ROOTFS")
	Expect(present).To(BeTrue(), "WINC_TEST_ROOTFS not set")

	wincBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc")
	Expect(err).ToNot(HaveOccurred())

	wincNetworkBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc-network")
	Expect(err).ToNot(HaveOccurred())

	wincImageBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc-image")
	Expect(err).ToNot(HaveOccurred())

	wincImageDir := filepath.Dir(wincImageBin)
	err = exec.Command("gcc.exe", "-c", "..\\..\\volume\\quota\\quota.c", "-o", filepath.Join(wincImageDir, "quota.o")).Run()
	Expect(err).NotTo(HaveOccurred())

	err = exec.Command("gcc.exe",
		"-shared",
		"-o", filepath.Join(wincImageDir, "quota.dll"),
		filepath.Join(wincImageDir, "quota.o"),
		"-lole32", "-loleaut32").Run()
	Expect(err).NotTo(HaveOccurred())

	serverBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc-network/fixtures/server")
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	var err error
	bundlePath, err = ioutil.TempDir("", "winccontainer")
	Expect(err).NotTo(HaveOccurred())

	subnetRange, gatewayAddress = randomSubnetAddress()

	conf := network.Config{
		SubnetRange:    subnetRange,
		NetworkName:    gatewayAddress,
		GatewayAddress: gatewayAddress,
	}

	content, err := json.Marshal(conf)
	Expect(err).NotTo(HaveOccurred())

	networkConfig, err := ioutil.TempFile("", "winc-network-config")
	Expect(err).NotTo(HaveOccurred())
	networkConfigFile = networkConfig.Name()

	_, err = networkConfig.Write(content)
	Expect(err).NotTo(HaveOccurred())

	Expect(networkConfig.Close()).To(Succeed())

	output, err := exec.Command(wincNetworkBin, "--action", "create", "--configFile", networkConfigFile).CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(output))
})

var _ = AfterEach(func() {
	Expect(os.RemoveAll(bundlePath)).To(Succeed())

	output, err := exec.Command(wincNetworkBin, "--action", "delete", "--configFile", networkConfigFile).CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(output))

	Expect(os.Remove(networkConfigFile)).To(Succeed())
})

func createSandbox(storePath, rootfsPath, containerId string) image.ImageSpec {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	cmd := exec.Command(wincImageBin, "--store", storePath, "create", rootfsPath, containerId)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	Expect(cmd.Run()).To(Succeed(), fmt.Sprintf("winc-image stdout: %s\n\n winc-image stderr: %s\n\n", stdOut.String(), stdErr.String()))
	var imageSpec image.ImageSpec
	Expect(json.Unmarshal(stdOut.Bytes(), &imageSpec)).To(Succeed())
	return imageSpec
}

func runtimeSpecGenerator(imageSpec image.ImageSpec, containerId string) specs.Spec {
	return specs.Spec{
		Version: specs.Version,
		Process: &specs.Process{
			Args: []string{"powershell"},
			Cwd:  "/",
		},
		Root: &specs.Root{
			Path: imageSpec.RootFs,
		},
		Windows: &specs.Windows{
			LayerFolders: imageSpec.Windows.LayerFolders,
		},
	}
}

func randomSubnetAddress() (string, string) {
	for {
		subnet, gateway := randomValidSubnetAddress()
		_, err := hcsshim.GetHNSNetworkByName(subnet)
		if err != nil {
			Expect(err).To(MatchError(ContainSubstring("Network " + subnet + " not found")))
			return subnet, gateway
		}
	}
}

func randomValidSubnetAddress() (string, string) {
	octet1 := rand.Intn(maxStandardIPOctet)
	gatewayAddress := fmt.Sprintf("172.0.%d.1", octet1)
	subnet := fmt.Sprintf("172.0.%d.0/30", octet1)
	return subnet, gatewayAddress
}

func getContainerState(containerId string) specs.State {
	stdOut, _, err := execute(exec.Command(wincBin, "state", containerId))
	Expect(err).ToNot(HaveOccurred())

	var state specs.State
	Expect(json.Unmarshal(stdOut.Bytes(), &state)).To(Succeed())
	return state
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}

func execute(c *exec.Cmd) (*bytes.Buffer, *bytes.Buffer, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	c.Stdout = io.MultiWriter(stdOut, GinkgoWriter)
	c.Stderr = io.MultiWriter(stdErr, GinkgoWriter)
	err := c.Run()

	return stdOut, stdErr, err
}

func allEndpoints(containerID string) []string {
	container, err := hcsshim.OpenContainer(containerID)
	Expect(err).To(Succeed())

	stats, err := container.Statistics()
	Expect(err).To(Succeed())

	var endpointIDs []string
	for _, network := range stats.Network {
		endpointIDs = append(endpointIDs, network.EndpointId)
	}

	return endpointIDs
}

func containerExists(containerId string) bool {
	query := hcsshim.ComputeSystemQuery{
		Owners: []string{"winc"},
		IDs:    []string{containerId},
	}
	containers, err := hcsshim.GetContainers(query)
	Expect(err).ToNot(HaveOccurred())
	return len(containers) > 0
}
