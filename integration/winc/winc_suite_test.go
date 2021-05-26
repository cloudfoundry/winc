package main_test

import (
	"encoding/json"
	"fmt"
	mathrand "math/rand"
	"os"
	"os/exec"
	"strconv"
	"time"
	"io/ioutil"
	"path/filepath"

	testhelpers "code.cloudfoundry.org/winc/integration/helpers"
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
	consumeBin      string
	sleepBin        string
	goshutBin       string
	helpers         *testhelpers.Helpers
	debug           bool
	failed          bool
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
		Pids struct {
			Current uint64 `json:"current,omitempty"`
			Limit   uint64 `json:"limit,omitempty"`
		} `json:"pids"`
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

	debug, _ = strconv.ParseBool(os.Getenv("DEBUG"))

	wincBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc")
	Expect(err).ToNot(HaveOccurred())

	consumeBin, err = gexec.Build("code.cloudfoundry.org/winc/integration/winc/fixtures/consume")
	Expect(err).ToNot(HaveOccurred())

	sleepBin, err = gexec.Build("code.cloudfoundry.org/winc/integration/winc/fixtures/sleep")
	Expect(err).ToNot(HaveOccurred())

	goshutBin, err = gexec.Build("code.cloudfoundry.org/winc/integration/winc/fixtures/goshut")
	Expect(err).ToNot(HaveOccurred())

	sleepDir, err := ioutil.TempDir("", "winccontainer")
	Expect(err).ToNot(HaveOccurred())

	err = os.Rename(sleepBin, filepath.Join(sleepDir, "sleep.exe"))
	Expect(err).ToNot(HaveOccurred())

	helpers = testhelpers.NewHelpers(wincBin, grootBin, grootImageStore, "", debug)
})

var _ = AfterSuite(func() {
	if failed && debug {
		fmt.Println(string(helpers.Logs()))
	}
	gexec.CleanupBuildArtifacts()
})

func processSpecGenerator() specs.Process {
	return specs.Process{
		Cwd:  "C:\\Windows",
		Args: []string{"cmd.exe"},
		Env:  []string{"var1=foo", "var2=bar"},
	}
}

/*
* This test was required to test CTRL+C behavior
* Might require this function very soon when it's
* re-implemented

func sendCtrlBreak(s *gexec.Session) {
	d, err := syscall.LoadDLL("kernel32.dll")
	Expect(err).ToNot(HaveOccurred())
	p, err := d.FindProc("GenerateConsoleCtrlEvent")
	Expect(err).ToNot(HaveOccurred())
	r, _, err := p.Call(syscall.CTRL_BREAK_EVENT, uintptr(s.Command.Process.Pid))
	Expect(r).ToNot(Equal(0), fmt.Sprintf("GenerateConsoleCtrlEvent: %v\n", err))
}
*/

func getStats(containerId string) wincStats {
	var stats wincStats
	stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "events", "--stats", containerId))
	Expect(err).To(Succeed(), stdOut.String(), stdErr.String())
	Expect(json.Unmarshal(stdOut.Bytes(), &stats)).To(Succeed())
	return stats
}
