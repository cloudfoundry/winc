package main_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/winc/image"
	"code.cloudfoundry.org/winc/network"

	"github.com/Microsoft/hcsshim"
	ps "github.com/mitchellh/go-ps"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"testing"
)

const (
	defaultTimeout  = time.Second * 10
	defaultInterval = time.Millisecond * 200
	rootPath        = "C:\\run\\winc"
)

var (
	wincBin            string
	wincNetworkBin     string
	wincImageBin       string
	rootfsPath         string
	bundlePath         string
	suiteNetConfigFile string
	readBin            string
	consumeBin         string
)

func TestWinc(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(defaultTimeout)
	SetDefaultEventuallyPollingInterval(defaultInterval)
	RunSpecs(t, "Winc Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	var (
		present bool
		err     error
	)

	rootfsPath, present = os.LookupEnv("WINC_TEST_ROOTFS")
	Expect(present).To(BeTrue(), "WINC_TEST_ROOTFS not set")

	wincBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc")
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

	wincNetworkBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc-network")
	Expect(err).ToNot(HaveOccurred())

	consumeBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc/fixtures/consume")
	Expect(err).ToNot(HaveOccurred())
	readBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc/fixtures/read")
	Expect(err).ToNot(HaveOccurred())

	configDir, err := ioutil.TempDir("", "winc-network.integration.suite.config")
	Expect(err).ToNot(HaveOccurred())

	suiteNetConfigFile = filepath.Join(configDir, "winc-network.json")

	createWincNATNetwork()

	return []byte(strings.Join([]string{wincBin, wincImageBin, rootfsPath, consumeBin, readBin}, "^"))

}, func(setupPaths []byte) {
	paths := strings.Split(string(setupPaths), "^")
	wincBin = paths[0]
	wincImageBin = paths[1]
	rootfsPath = paths[2]
	consumeBin = paths[3]
	readBin = paths[4]
})

var _ = SynchronizedAfterSuite(func() {
	//noop
}, func() {
	deleteWincNATNetwork()
	Expect(os.RemoveAll(filepath.Dir(suiteNetConfigFile))).To(Succeed())
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	var err error
	bundlePath, err = ioutil.TempDir("", "winccontainer")
	Expect(err).To(Succeed())
})

var _ = AfterEach(func() {
	Expect(os.RemoveAll(bundlePath)).To(Succeed())
})

func createWincNATNetwork() {
	insiderPreview := os.Getenv("INSIDER_PREVIEW") != ""
	// default config follows:
	conf := network.Config{
		InsiderPreview: insiderPreview,
		NetworkName:    "winc-nat",
		SubnetRange:    "172.30.0.0/22",
		GatewayAddress: "172.30.0.1",
	}

	c, err := json.Marshal(conf)
	Expect(err).NotTo(HaveOccurred())
	Expect(ioutil.WriteFile(suiteNetConfigFile, c, 0644)).To(Succeed())

	_, err = hcsshim.GetHNSNetworkByName("winc-nat")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(Equal("Network winc-nat not found"))

	output, err := exec.Command(wincNetworkBin, "--action", "create", "--configFile", suiteNetConfigFile).CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(output))

	net, err := hcsshim.GetHNSNetworkByName(conf.NetworkName)
	Expect(err).ToNot(HaveOccurred())
	Expect(net.Name).To(Equal(conf.NetworkName))

	Expect(len(net.Subnets)).To(Equal(1))
	Expect(net.Subnets[0].AddressPrefix).To(Equal(conf.SubnetRange))
	Expect(net.Subnets[0].GatewayAddress).To(Equal(conf.GatewayAddress))
}

func deleteWincNATNetwork() {
	output, err := exec.Command(wincNetworkBin, "--action", "delete", "--configFile", suiteNetConfigFile).CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(output))

	_, err = hcsshim.GetHNSNetworkByName("winc-nat")
	Expect(err.Error()).To(Equal("Network winc-nat not found"))
}

func getContainerState(containerId string) specs.State {
	stdOut, _, err := execute(exec.Command(wincBin, "state", containerId))
	Expect(err).ToNot(HaveOccurred())

	var state specs.State
	Expect(json.Unmarshal(stdOut.Bytes(), &state)).To(Succeed())
	return state
}

func createSandbox(storePath, rootfsPath, containerId string) image.ImageSpec {
	stdOut := new(bytes.Buffer)
	cmd := exec.Command(wincImageBin, "--store", storePath, "create", rootfsPath, containerId)
	cmd.Stdout = stdOut
	Expect(cmd.Run()).To(Succeed(), "winc-image output: "+stdOut.String())
	var imageSpec image.ImageSpec
	Expect(json.Unmarshal(stdOut.Bytes(), &imageSpec)).To(Succeed())
	return imageSpec
}

func runtimeSpecGenerator(imageSpec image.ImageSpec) specs.Spec {
	return specs.Spec{
		Version: specs.Version,
		Process: &specs.Process{
			Args: []string{"powershell"},
			Cwd:  "C:\\",
		},
		Root: &specs.Root{
			Path: imageSpec.RootFs,
		},
		Windows: &specs.Windows{
			LayerFolders: imageSpec.Windows.LayerFolders,
		},
	}
}

func processSpecGenerator() specs.Process {
	return specs.Process{
		Cwd:  "C:\\Windows",
		Args: []string{"cmd.exe"},
		Env:  []string{"var1=foo", "var2=bar"},
	}
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

func containerProcesses(containerId, filter string) []hcsshim.ProcessListItem {
	container, err := hcsshim.OpenContainer(containerId)
	Expect(err).To(Succeed())

	pl, err := container.ProcessList()
	Expect(err).To(Succeed())

	if filter != "" {
		var filteredPL []hcsshim.ProcessListItem
		for _, v := range pl {
			if v.ImageName == filter {
				filteredPL = append(filteredPL, v)
			}
		}

		return filteredPL
	}

	return pl
}

func randomContainerId() string {
	max := big.NewInt(math.MaxInt64)
	r, err := rand.Int(rand.Reader, max)
	Expect(err).NotTo(HaveOccurred())

	return fmt.Sprintf("%d", r.Int64())
}

func isParentOf(parentPid, childPid int) bool {
	var (
		process ps.Process
		err     error
	)

	var foundParent bool
	for {
		process, err = ps.FindProcess(childPid)
		Expect(err).To(Succeed())

		if process == nil {
			break
		}
		if process.PPid() == parentPid {
			foundParent = true
			break
		}
		childPid = process.PPid()
	}

	return foundParent
}

func copy(dst, src string) error {
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
