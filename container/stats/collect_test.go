package stats_test

import (
	"errors"

	"code.cloudfoundry.org/winc/container/stats"
	"code.cloudfoundry.org/winc/container/stats/fakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Collect", func() {
	var fakeContainer *fakes.Container

	BeforeEach(func() {
		fakeContainer = &fakes.Container{}
	})

	It("returns the relevant container statistics", func() {
		s, err := stats.Collect(fakeContainer)
		Expect(err).NotTo(HaveOccurred())
		Expect(s).To(Equal(""))
	})

	Context("getting the container statistics fails", func() {
		BeforeEach(func() {
			fakeContainer.StatisticsReturns(hcsshim.Statistics{}, errors.New("getting statistics failed"))
		})

		It("returns a descriptive error", func() {
			_, err := stats.Collect(fakeContainer)
			Expect(err).To(MatchError("something descriptive"))
		})
	})
})
