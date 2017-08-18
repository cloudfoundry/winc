package main_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/winc/image"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"testing"
)

const defaultTimeout = time.Second * 10
const defaultInterval = time.Millisecond * 200

var (
	wincBin        string
	wincNetworkBin string
	wincImageBin   string
	rootfsPath     string
	bundlePath     string
)

func TestWincNetwork(t *testing.T) {
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

		wincNetworkBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc-network")
		Expect(err).ToNot(HaveOccurred())
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

	RunSpecs(t, "Winc-Network Suite")
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
