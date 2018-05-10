package container_test

import (
	"errors"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/winc/runtime/container"
	"code.cloudfoundry.org/winc/runtime/container/fakes"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Stats", func() {
	const containerId = "some-stats-container"
	var (
		bundlePath       string
		hcsClient        *fakes.HCSClient
		containerManager *container.Manager
		fakeContainer    *hcsfakes.Container
	)

	BeforeEach(func() {
		hcsClient = &fakes.HCSClient{}
		logger := (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "stats")

		containerManager = container.New(logger, hcsClient, containerId)

		fakeContainer = &hcsfakes.Container{}
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
			fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{hcsshim.ProcessListItem{}}, nil)
		})

		It("returns the correct container stats values", func() {
			stats, err := containerManager.Stats()
			Expect(err).ToNot(HaveOccurred())

			Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
			Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

			expectedStats := container.Statistics{}
			expectedStats.Data.Memory.Raw.TotalRss = 666
			expectedStats.Data.CPUStats.CPUUsage.Usage = 12300
			expectedStats.Data.CPUStats.CPUUsage.System = 10100
			expectedStats.Data.CPUStats.CPUUsage.User = 2200
			expectedStats.Data.Pids.Current = 1
			expectedStats.Data.Pids.Limit = 0
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
