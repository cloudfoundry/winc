package main_test

import (
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	testhelpers "code.cloudfoundry.org/winc/integration/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

const (
	defaultTimeout  = time.Second * 10
	defaultInterval = time.Millisecond * 200
	imageStore      = "C:\\run\\winc"
)

var (
	wincBin        string
	wincNetworkBin string
	wincImageBin   string
	serverBin      string
	netoutBin      string
	rootfsPath     string
	helpers        *testhelpers.Helpers
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
	err = exec.Command("gcc.exe", "-c", "..\\..\\image\\volume\\quota\\quota.c", "-o", filepath.Join(wincImageDir, "quota.o")).Run()
	Expect(err).NotTo(HaveOccurred())

	err = exec.Command("gcc.exe",
		"-shared",
		"-o", filepath.Join(wincImageDir, "quota.dll"),
		filepath.Join(wincImageDir, "quota.o"),
		"-lole32", "-loleaut32").Run()
	Expect(err).NotTo(HaveOccurred())

	serverBin, err = gexec.Build("code.cloudfoundry.org/winc/integration/winc-network/fixtures/server")
	Expect(err).ToNot(HaveOccurred())

	netoutBin, err = gexec.Build("code.cloudfoundry.org/winc/integration/winc-network/fixtures/netout")
	Expect(err).ToNot(HaveOccurred())

	helpers = testhelpers.NewHelpers(wincBin, wincImageBin, wincNetworkBin)
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
