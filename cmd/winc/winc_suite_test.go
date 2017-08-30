package main_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/winc/image"

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
	wincBin      string
	wincImageBin string
	readBin      string
	consumeBin   string
	rootfsPath   string
	bundlePath   string
)

func TestWinc(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(defaultTimeout)
	SetDefaultEventuallyPollingInterval(defaultInterval)

	BeforeSuite(func() {
		var (
			err     error
			present bool
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

		consumeBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc/fixtures/consume")
		Expect(err).ToNot(HaveOccurred())
		readBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc/fixtures/read")
		Expect(err).ToNot(HaveOccurred())
		rand.Seed(time.Now().UnixNano())
	})

	AfterSuite(func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		var err error
		bundlePath, err = ioutil.TempDir("", "winccontainer")
		Expect(err).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	RunSpecs(t, "Winc Suite")
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

func runtimeSpecGenerator(imageSpec image.ImageSpec, containerId string) specs.Spec {
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
