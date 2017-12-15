package perf_test

import (
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

const (
	defaultTimeout              = time.Second * 10
	defaultInterval             = time.Millisecond * 200
	defaultConcurrentContainers = 15
)

var (
	wincBin              string
	wincNetworkBin       string
	wincImageBin         string
	rootfsPath           string
	concurrentContainers int
)

func TestPerf(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(defaultTimeout)
	SetDefaultEventuallyPollingInterval(defaultInterval)
	RunSpecs(t, "Perf Suite")
}

var _ = BeforeSuite(func() {
	var (
		present bool
		err     error
	)

	rand.Seed(time.Now().UnixNano() + int64(GinkgoParallelNode()))

	rootfsPath, present = os.LookupEnv("WINC_TEST_ROOTFS")
	Expect(present).To(BeTrue(), "WINC_TEST_ROOTFS not set")

	concurrentContainers = defaultConcurrentContainers
	concurrentContainersStr, present := os.LookupEnv("WINC_TEST_PERF_CONCURRENT_CONTAINERS")
	if present {
		concurrentContainers, err = strconv.Atoi(concurrentContainersStr)
		Expect(err).ToNot(HaveOccurred())
	}

	wincBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc")
	Expect(err).ToNot(HaveOccurred())

	wincNetworkBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc-network")
	Expect(err).ToNot(HaveOccurred())

	wincImageBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc-image")
	Expect(err).ToNot(HaveOccurred())

	wincImageDir := filepath.Dir(wincImageBin)
	err = exec.Command("gcc.exe", "-c", "..\\volume\\quota\\quota.c", "-o", filepath.Join(wincImageDir, "quota.o")).Run()
	Expect(err).NotTo(HaveOccurred())

	err = exec.Command("gcc.exe",
		"-shared",
		"-o", filepath.Join(wincImageDir, "quota.dll"),
		filepath.Join(wincImageDir, "quota.o"),
		"-lole32", "-loleaut32").Run()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
