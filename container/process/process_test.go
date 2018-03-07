package process_test

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"code.cloudfoundry.org/winc/container/process"
	"code.cloudfoundry.org/winc/container/state/fakes"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("ProcessManager", func() {
	const (
		containerId = "some-container-id"
		bundlePath  = "some-bundle-path"
	)

	var (
		hcsClient *fakes.HCSClient
		rootDir   string
		pm        *process.Manager
		container *hcsfakes.Container
	)

	BeforeEach(func() {
		var err error

		rootDir, err = ioutil.TempDir("", "winc.container.state.test")
		Expect(err).NotTo(HaveOccurred())

		hcsClient = &fakes.HCSClient{}
		pm = process.NewManager(hcsClient)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(rootDir)).To(Succeed())
	})

	Context("ContainerPid", func() {
		Context("when there are hjjjno wininit.exe processes in the container", func() {
			BeforeEach(func() {
				container = &hcsfakes.Container{}
				hcsClient.OpenContainerReturnsOnCall(0, container, nil)
				container.ProcessListReturnsOnCall(0, []hcsshim.ProcessListItem{}, nil)
			})

			It("returns 0 as the pid", func() {
				pid, err := pm.ContainerPid(containerId)
				Expect(err).NotTo(HaveOccurred())
				Expect(pid).To(Equal(0))
			})
		})

		Context("when there are multiple wininit.exe processes in the container", func() {
			BeforeEach(func() {
				container = &hcsfakes.Container{}
				hcsClient.OpenContainerReturnsOnCall(0, container, nil)
				now := time.Now()
				container.ProcessListReturnsOnCall(0, []hcsshim.ProcessListItem{
					{ProcessId: 668, ImageName: "wininit.exe", CreateTimestamp: now.AddDate(0, -1, 0)},
					{ProcessId: 667, ImageName: "wininit.exe", CreateTimestamp: now.AddDate(0, -2, 0)},
					{ProcessId: 666, ImageName: "wininit.exe", CreateTimestamp: now},
				}, nil)
			})

			It("returns the pid of the oldest one as the container pid", func() {
				pid, err := pm.ContainerPid(containerId)
				Expect(err).NotTo(HaveOccurred())
				Expect(pid).To(Equal(667))
			})
		})

		Context("when getting container pid fails", func() {
			BeforeEach(func() {
				hcsClient.OpenContainerReturns(nil, errors.New("couldn't get pid"))
				hcsClient.GetContainerPropertiesReturnsOnCall(1, hcsshim.ContainerProperties{Stopped: false}, nil)
			})

			It("returns an error", func() {
				_, err := pm.ContainerPid(containerId)
				//TODO: use more specific error
				Expect(err).To(MatchError("couldn't get pid"))
			})
		})

	})

	Context("ProcessStartTime", func() {
		var session *gexec.Session

		BeforeEach(func() {
			var err error
			cmd := exec.Command("cmd.exe", "/C", "waitfor /t 9999 forever")
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			session.Kill()
		})

		It("returns the start time for the process", func() {
			startTime, err := pm.ProcessStartTime(uint32(session.Command.Process.Pid))
			Expect(err).ToNot(HaveOccurred())
			Expect(startTime.LowDateTime).To(BeNumerically(">", 0))
			Expect(startTime.HighDateTime).To(BeNumerically(">", 0))
		})

		Context("when the pid does not exist", func() {
			It("", func() {
				pid := uint32(session.Command.Process.Pid)
				session.Kill()
				Eventually(session).Should(gexec.Exit())
				_, err := pm.ProcessStartTime(pid)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("IsProcessRunning", func() {
		It("", func() {
			containerPid = 123
			otherProcessPid = 456
			//							processList := []hcsshim.ProcessListItem{
			//								hcsshim.ProcessListItem{
			//									ProcessId: uint32(containerPid),
			//									ImageName: "wininit.exe",
			//								},
			//								hcsshim.ProcessListItem{
			//									ProcessId: uint32(initProcessPid),
			//									ImageName: "init-process.exe",
			//								},
			//							}
			//							container.ProcessListReturnsOnCall(0, processList, nil)
			//							container.ProcessListReturnsOnCall(1, processList, nil)
			result, err = pm.IsProcessRunning(containerPid)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeTrue())

		})
	})
})
