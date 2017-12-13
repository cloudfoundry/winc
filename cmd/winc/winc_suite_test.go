package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	mathrand "math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	helpers "code.cloudfoundry.org/winc/cmd/helpers"
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
	imageStore      = "C:\\run\\winc"
)

var (
	wincBin      string
	wincImageBin string
	rootfsPath   string
	readBin      string
	consumeBin   string
	sleepBin     string
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
	sleepBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc/fixtures/sleep")
	Expect(err).ToNot(HaveOccurred())
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

func generateBundle(bundleSpec specs.Spec, bundlePath, id string) {
	config, err := json.Marshal(&bundleSpec)
	Expect(err).NotTo(HaveOccurred())
	configFile := filepath.Join(bundlePath, "config.json")
	Expect(ioutil.WriteFile(configFile, config, 0666)).To(Succeed())
}

func wincBinGenericCreate(bundleSpec specs.Spec, bundlePath, containerId string) {
	generateBundle(bundleSpec, bundlePath, containerId)
	stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, containerId))
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
}

func wincBinGenericExecInContainer(containerId string, args []string) *bytes.Buffer {
	stdOut, stdErr, err := helpers.ExecInContainer(wincBin, containerId, args, false)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())
	return stdOut
}
