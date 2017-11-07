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

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"testing"
)

const (
	defaultTimeout  = time.Second * 10
	defaultInterval = time.Millisecond * 200
)

var (
	wincBin        string
	wincNetworkBin string
	wincImageBin   string
	serverBin      string
	rootfsPath     string
	bundlePath     string
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
	rootfsPath, present = os.LookupEnv("WINC_TEST_ROOTFS")
	Expect(present).To(BeTrue(), "WINC_TEST_ROOTFS not set")

	var err error
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
})

var _ = AfterEach(func() {
	Expect(os.RemoveAll(bundlePath)).To(Succeed())
})

func createSandbox(storePath, rootfsPath, containerId string) specs.Spec {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	cmd := exec.Command(wincImageBin, "--store", storePath, "create", rootfsPath, containerId)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	Expect(cmd.Run()).To(Succeed(), fmt.Sprintf("winc-image stdout: %s\n\n winc-image stderr: %s\n\n", stdOut.String(), stdErr.String()))
	var spec specs.Spec
	Expect(json.Unmarshal(stdOut.Bytes(), &spec)).To(Succeed())
	return spec
}

func runtimeSpecGenerator(baseSpec specs.Spec, containerId string) specs.Spec {
	return specs.Spec{
		Version: specs.Version,
		Process: &specs.Process{
			Args: []string{"powershell"},
			Cwd:  "/",
		},
		Root: &specs.Root{
			Path: baseSpec.Root.Path,
		},
		Windows: &specs.Windows{
			LayerFolders: baseSpec.Windows.LayerFolders,
		},
	}
}

func getContainerState(containerId string) specs.State {
	stdOut, _, err := execute(wincBin, "state", containerId)
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

func execute(cmd string, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	command := exec.Command(cmd, args...)
	command.Stdout = io.MultiWriter(stdOut, GinkgoWriter)
	command.Stderr = io.MultiWriter(stdErr, GinkgoWriter)
	err := command.Run()
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
