package main_test

import (
	"io/ioutil"
	"os"
	"runtime"
	"time"

	"code.cloudfoundry.org/winc/hcsclient"

	"github.com/Microsoft/hcsshim"
	ps "github.com/mitchellh/go-ps"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"testing"
)

const defaultTimeout = time.Second * 10
const defaultInterval = time.Millisecond * 200

var (
	wincBin    string
	rootfsPath string
	bundlePath string
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

func runtimeSpecGenerator(rootfsPath string) specs.Spec {
	return specs.Spec{
		Version: specs.Version,
		Platform: specs.Platform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		Process: &specs.Process{
			Args: []string{"powershell"},
			Cwd:  "/",
		},
		Root: specs.Root{
			Path: rootfsPath,
		},
	}
}

func processSpecGenerator() specs.Process {
	return specs.Process{
		Cwd:  "C:\\Windows",
		Args: []string{"cmd.exe"},
		Env:  []string{"var1=foo", "var2=bar"},
		User: specs.User{
			Username: "Administrator",
		},
	}
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

func containerProcesses(client hcsclient.Client, containerId, filter string) []hcsshim.ProcessListItem {
	container, err := client.OpenContainer(containerId)
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
