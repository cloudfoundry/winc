package container_test

import (
	"errors"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/containerfakes"
	"code.cloudfoundry.org/winc/hcs/hcsfakes"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stats", func() {
	var (
		bundlePath       string
		hcsClient        *containerfakes.FakeHCSClient
		mounter          *containerfakes.FakeMounter
		containerManager *container.Manager
		fakeContainer    *hcsfakes.FakeContainer
	)

	BeforeEach(func() {
		var err error
		bundlePath, err = ioutil.TempDir("", "bundlePath")
		Expect(err).ToNot(HaveOccurred())

		hcsClient = &containerfakes.FakeHCSClient{}
		mounter = &containerfakes.FakeMounter{}
		containerManager = container.NewManager(hcsClient, mounter, "", bundlePath)
		fakeContainer = &hcsfakes.FakeContainer{}
		hcsClient.OpenContainerReturns(fakeContainer, nil)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Context("when the specified container exists", func() {
		BeforeEach(func() {
			fakeContainer.StatisticsReturns(hcsshim.Statistics{
				Memory: hcsshim.MemoryStats{
					UsageCommitBytes: 666,
				},
				Processor: hcsshim.ProcessorStats{
					TotalRuntime100ns:  123,
					RuntimeKernel100ns: 101,
					RuntimeUser100ns:   22,
				},
			}, nil)
		})

		It("returns the correct container stats values", func() {
			stats, err := containerManager.Stats()
			Expect(err).ToNot(HaveOccurred())

			expectedStats := container.Statistics{}
			expectedStats.Data.Memory.Raw.TotalRss = 666
			expectedStats.Data.CPUStats.CPUUsage.Usage = 12300
			expectedStats.Data.CPUStats.CPUUsage.System = 10100
			expectedStats.Data.CPUStats.CPUUsage.User = 2200
			Expect(stats).To(Equal(expectedStats))
		})
	})

	Context("when the container does not exist", func() {
		var openContainerError = errors.New("open container failed")

		BeforeEach(func() {
			hcsClient.OpenContainerReturns(nil, openContainerError)
		})

		It("errors", func() {
			_, err := containerManager.Stats()
			Expect(err).To(Equal(openContainerError))
		})
	})

	Context("when getting the stats fails", func() {
		var statsError = errors.New("stats failed")

		BeforeEach(func() {
			fakeContainer.StatisticsReturns(hcsshim.Statistics{}, statsError)
		})

		It("errors", func() {
			_, err := containerManager.Stats()
			Expect(err).To(Equal(statsError))
		})
	})
})
