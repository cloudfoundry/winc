package process_test

import (
	"errors"
	"io/ioutil"
	"os"
	"time"

	"code.cloudfoundry.org/winc/container/process"
	"code.cloudfoundry.org/winc/container/state/fakes"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		Context("when there are no wininit.exe processes in the container", func() {
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
		It("returns the start time for the process", func() {
			panic("IMPLEMENT ME")
		})
	})
})
