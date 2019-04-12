package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	testhelpers "code.cloudfoundry.org/winc/integration/helpers"
	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/netrules"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

const (
	defaultTimeout  = time.Second * 10
	defaultInterval = time.Millisecond * 200
)

var (
	wincBin           string
	wincNetworkBin    string
	grootBin          string
	grootImageStore   string
	serverBin         string
	netoutBin         string
	clientBin         string
	rootfsURI         string
	helpers           *testhelpers.Helpers
	containerId       string
	bundlePath        string
	tempDir           string
	networkConfigFile string
	networkConfig     network.Config
	windowsBuild      int
	debug             bool
	failed            bool
)

func TestWincNetwork(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(defaultTimeout)
	SetDefaultEventuallyPollingInterval(defaultInterval)
	RunSpecs(t, "Winc-Network Suite")
}

var _ = BeforeSuite(func() {
	rand.Seed(time.Now().UnixNano() + int64(GinkgoParallelNode()))

	var present bool
	rootfsURI, present = os.LookupEnv("WINC_TEST_ROOTFS")
	Expect(present).To(BeTrue(), "WINC_TEST_ROOTFS not set")

	grootBin, present = os.LookupEnv("GROOT_BINARY")
	Expect(present).To(BeTrue(), "GROOT_BINARY not set")

	grootImageStore, present = os.LookupEnv("GROOT_IMAGE_STORE")
	Expect(present).To(BeTrue(), "GROOT_IMAGE_STORE not set")

	debug, _ = strconv.ParseBool(os.Getenv("DEBUG"))

	var err error
	wincBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc")
	Expect(err).ToNot(HaveOccurred())

	output, err := exec.Command("powershell", "-command", "[System.Environment]::OSVersion.Version.Build").CombinedOutput()
	Expect(err).NotTo(HaveOccurred())

	windowsBuild, err = strconv.Atoi(strings.TrimSpace(string(output)))
	Expect(err).NotTo(HaveOccurred())

	if windowsBuild == 16299 {
		// 1709
		wincNetworkBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc-network")
		Expect(err).ToNot(HaveOccurred())

		wincNetworkDir := filepath.Dir(wincNetworkBin)
		o, err := exec.Command("gcc.exe", "-c", "..\\..\\network\\firewall\\dll\\firewall.c", "-o", filepath.Join(wincNetworkDir, "firewall.o")).CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(o))

		err = exec.Command("gcc.exe",
			"-shared",
			"-o", filepath.Join(wincNetworkDir, "firewall.dll"),
			filepath.Join(wincNetworkDir, "firewall.o"),
			"-lole32", "-loleaut32").Run()
		Expect(err).NotTo(HaveOccurred())
	} else {
		// 1803 & 2019
		wincNetworkBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc-network", "-tags", "hnsAcls")
		Expect(err).ToNot(HaveOccurred())
	}

	serverBin, err = gexec.Build("code.cloudfoundry.org/winc/integration/winc-network/fixtures/server")
	Expect(err).ToNot(HaveOccurred())

	netoutBin, err = gexec.Build("code.cloudfoundry.org/winc/integration/winc-network/fixtures/netout")
	Expect(err).ToNot(HaveOccurred())

	clientBin, err = gexec.Build("code.cloudfoundry.org/winc/integration/winc-network/fixtures/client")
	Expect(err).ToNot(HaveOccurred())

	helpers = testhelpers.NewHelpers(wincBin, grootBin, grootImageStore, wincNetworkBin, debug)
})

var _ = AfterSuite(func() {
	if failed && debug {
		fmt.Println(string(helpers.Logs()))
	}
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	var err error
	tempDir, err = ioutil.TempDir("", "winc-network.config")
	Expect(err).NotTo(HaveOccurred())
	networkConfigFile = filepath.Join(tempDir, "winc-network.json")

	bundlePath, err = ioutil.TempDir("", "winccontainer")
	Expect(err).NotTo(HaveOccurred())
	containerId = filepath.Base(bundlePath)
})

var _ = AfterEach(func() {
	Expect(os.RemoveAll(tempDir)).To(Succeed())
	Expect(os.RemoveAll(bundlePath)).To(Succeed())
})

func uploadFile(containerId string, fileSize int, serverURL string) int {
	stdout, _, err := helpers.ExecInContainer(containerId, []string{"C:\\client.exe", serverURL, "upload", strconv.Itoa(fileSize)}, false)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	outputRegex := regexp.MustCompile(`uploaded in ([0-9]+) miliseconds`)
	match := outputRegex.FindStringSubmatch(strings.TrimSpace(stdout.String()))
	ExpectWithOffset(1, len(match)).To(Equal(2))
	uploadTime, err := strconv.Atoi(match[1])
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return uploadTime
}

func downloadFile(containerId string, fileSize int, serverURL string) int {
	stdout, _, err := helpers.ExecInContainer(containerId, []string{"C:\\client.exe", serverURL, "download", strconv.Itoa(fileSize)}, false)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	outputRegex := regexp.MustCompile(`downloaded in ([0-9]+) miliseconds`)
	match := outputRegex.FindStringSubmatch(strings.TrimSpace(stdout.String()))
	ExpectWithOffset(1, len(match)).To(Equal(2))
	downloadTime, err := strconv.Atoi(match[1])
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return downloadTime
}

func deleteContainerAndNetwork(id string, config network.Config) {
	helpers.NetworkDown(id, networkConfigFile)
	helpers.DeleteContainer(id)
	helpers.DeleteVolume(id)
	helpers.DeleteNetwork(config, networkConfigFile)
}

func getContainerIp(containerId string) net.IP {
	container, err := hcsshim.OpenContainer(containerId)
	Expect(err).NotTo(HaveOccurred(), "no containers with id: "+containerId)

	stats, err := container.Statistics()
	Expect(err).NotTo(HaveOccurred())

	Expect(stats.Network).NotTo(BeEmpty(), "container has no networks attached: "+containerId)
	endpoint, err := hcsshim.GetHNSEndpointByID(stats.Network[0].EndpointId)
	Expect(err).NotTo(HaveOccurred())

	return endpoint.IPAddress
}

func randomPort() int {
	l, err := net.Listen("tcp", ":0")
	Expect(err).NotTo(HaveOccurred())
	defer l.Close()
	split := strings.Split(l.Addr().String(), ":")
	port, err := strconv.Atoi(split[len(split)-1])
	Expect(err).NotTo(HaveOccurred())
	return port
}

func endpointExists(endpointName string) bool {
	_, err := hcsshim.GetHNSEndpointByName(endpointName)
	if err != nil {
		if _, ok := err.(hcsshim.EndpointNotFoundError); ok {
			return false
		}

		Expect(err).NotTo(HaveOccurred())
	}

	return true
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

func findExternalPort(portMappings, containerPort string) int {
	var mappedPorts []netrules.PortMapping
	err := json.Unmarshal([]byte(portMappings), &mappedPorts)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	var externalPort, internalPort int
	internalPort, err = strconv.Atoi(containerPort)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	for _, v := range mappedPorts {
		if v.ContainerPort == uint32(internalPort) {
			externalPort = int(v.HostPort)
			break
		}
	}
	ExpectWithOffset(1, externalPort).ToNot(Equal(0))
	return externalPort
}

func httpGetInto(address string, resp *http.Response) func() error {
	return func() error {
		r, err := http.Get(address)
		if r != nil {
			*resp = *r
		}
		return err
	}
}
