package main_test

import (
	"encoding/json"
	"fmt"
	mathrand "math/rand"
	"os"
	"os/exec"
	"syscall"
	"time"

	testhelpers "code.cloudfoundry.org/winc/integration/helpers"
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
	wincBin         string
	grootBin        string
	grootImageStore string
	rootfsURI       string
	readBin         string
	consumeBin      string
	sleepBin        string
	helpers         *testhelpers.Helpers
)

type wincStats struct {
	Data struct {
		CPUStats struct {
			CPUUsage struct {
				Usage  uint64 `json:"total"`
				System uint64 `json:"kernel"`
				User   uint64 `json:"user"`
			} `json:"usage"`
		} `json:"cpu"`
		Memory struct {
			Stats struct {
				TotalRss uint64 `json:"total_rss"`
			} `json:"raw"`
		} `json:"memory"`
	} `json:"data"`
}

func TestWinc(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(defaultTimeout)
	SetDefaultEventuallyPollingInterval(defaultInterval)
	RunSpecs(t, "Winc Suite")
}

var _ = BeforeSuite(func() {
	mathrand.Seed(time.Now().UnixNano() + int64(GinkgoParallelNode()))
	var (
		present bool
		err     error
	)

	rootfsURI, present = os.LookupEnv("WINC_TEST_ROOTFS")
	Expect(present).To(BeTrue(), "WINC_TEST_ROOTFS not set")

	grootBin, present = os.LookupEnv("GROOT_BINARY")
	Expect(present).To(BeTrue(), "GROOT_BINARY not set")

	grootImageStore, present = os.LookupEnv("GROOT_IMAGE_STORE")
	Expect(present).To(BeTrue(), "GROOT_IMAGE_STORE not set")

	wincBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc")
	Expect(err).ToNot(HaveOccurred())

	consumeBin, err = gexec.Build("code.cloudfoundry.org/winc/integration/winc/fixtures/consume")
	Expect(err).ToNot(HaveOccurred())
	readBin, err = gexec.Build("code.cloudfoundry.org/winc/integration/winc/fixtures/read")
	Expect(err).ToNot(HaveOccurred())
	sleepBin, err = gexec.Build("code.cloudfoundry.org/winc/integration/winc/fixtures/sleep")
	Expect(err).ToNot(HaveOccurred())

	helpers = testhelpers.NewHelpers(wincBin, grootBin, grootImageStore, "")
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func processSpecGenerator() specs.Process {
	return specs.Process{
		Cwd:  "C:\\Windows",
		Args: []string{"cmd.exe"},
		Env:  []string{"var1=foo", "var2=bar"},
	}
}

func containerProcesses(containerId, filter string) []hcsshim.ProcessListItem {
	container, err := hcsshim.OpenContainer(containerId)
	Expect(err).To(Succeed())

	pl, err := container.ProcessList()
	Expect(err).To(Succeed())

	if filter != "" {
		var filteredPL []hcsshim.ProcessListItem
		for _, v := range pl {
			fmt.Println(v.ImageName)
			if v.ImageName == filter {
				filteredPL = append(filteredPL, v)
			}
		}

		return filteredPL
	}

	return pl
}

func sendCtrlBreak(s *gexec.Session) {
	d, err := syscall.LoadDLL("kernel32.dll")
	Expect(err).ToNot(HaveOccurred())
	p, err := d.FindProc("GenerateConsoleCtrlEvent")
	Expect(err).ToNot(HaveOccurred())
	r, _, err := p.Call(syscall.CTRL_BREAK_EVENT, uintptr(s.Command.Process.Pid))
	Expect(r).ToNot(Equal(0), fmt.Sprintf("GenerateConsoleCtrlEvent: %v\n", err))
}

func getStats(containerId string) wincStats {
	var stats wincStats
	stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "events", "--stats", containerId))
	Expect(err).To(Succeed(), stdOut.String(), stdErr.String())
	Expect(json.Unmarshal(stdOut.Bytes(), &stats)).To(Succeed())
	return stats
}
