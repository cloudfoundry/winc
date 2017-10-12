package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

const defaultTimeout = time.Second * 10
const defaultInterval = time.Millisecond * 200

var (
	wincBin            string
	wincNetworkBin     string
	wincImageBin       string
	rootfsPath         string
	bundlePath         string
	suiteNetConfigFile string
)

func TestWincNetwork(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(defaultTimeout)
	SetDefaultEventuallyPollingInterval(defaultInterval)
	RunSpecs(t, "Winc-Network Suite")
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

	configDir, err := ioutil.TempDir("", "winc-network.integration.suite.config")
	Expect(err).ToNot(HaveOccurred())

	suiteNetConfigFile = filepath.Join(configDir, "winc-network.json")

	createWincNATNetwork()

	return []byte(strings.Join([]string{wincBin, wincNetworkBin, wincImageBin, rootfsPath}, "^"))

}, func(setupPaths []byte) {
	paths := strings.Split(string(setupPaths), "^")
	wincBin = paths[0]
	wincNetworkBin = paths[1]
	wincImageBin = paths[2]
	rootfsPath = paths[3]
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
