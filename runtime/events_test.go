package runtime_test

import (
	"errors"

	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/runtime"
	"code.cloudfoundry.org/winc/runtime/container"
	"code.cloudfoundry.org/winc/runtime/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Events", func() {
	const (
		bundlePath  = "some/dir"
		rootDir     = "dir-for-state-and-things"
		containerId = "container-for-stats"
	)
	var (
		mounter          *fakes.Mounter
		stateFactory     *fakes.StateFactory
		sm               *fakes.StateManager
		containerFactory *fakes.ContainerFactory
		cm               *fakes.ContainerManager
		processWrapper   *fakes.ProcessWrapper
		p                *fakes.WrappedProcess
		hcsQuery         *fakes.HCSQuery
		r                *runtime.Runtime
		spec             *specs.Spec
		output           *gbytes.Buffer
	)

	BeforeEach(func() {
		mounter = &fakes.Mounter{}
		hcsQuery = &fakes.HCSQuery{}
		stateFactory = &fakes.StateFactory{}
		sm = &fakes.StateManager{}
		containerFactory = &fakes.ContainerFactory{}
		cm = &fakes.ContainerManager{}
		processWrapper = &fakes.ProcessWrapper{}
		p = &fakes.WrappedProcess{}
		spec = &specs.Spec{}

		stateFactory.NewManagerReturns(sm)
		containerFactory.NewManagerReturns(cm)

		output = gbytes.NewBuffer()

		r = runtime.New(stateFactory, containerFactory, mounter, hcsQuery, processWrapper, rootDir)
	})

	Context("show stats is true", func() {
		var expectedJSON string
		BeforeEach(func() {
			expectedJSON = `{
  "data": {
    "cpu": {
      "usage": {
        "total": 0,
        "kernel": 0,
        "user": 0
      }
    },
    "memory": {
      "raw": {}
    }
  }
}`
		})

		It("writes the stats to the output", func() {
			Expect(r.Events(containerId, output, true)).To(Succeed())
			Expect(string(output.Contents())).To(Equal(expectedJSON))

			_, c, id := containerFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(id).To(Equal(containerId))
		})

		Context("events is passed a nil io.Writer", func() {
			It("returns an error", func() {
				err := r.Events(containerId, nil, true)
				Expect(err).To(MatchError("provided output is nil"))
			})
		})
	})

	Context("show stats is false", func() {
		It("calls cm.Stats but doesn't write anything", func() {
			Expect(r.Events(containerId, output, false)).To(Succeed())
			Expect(string(output.Contents())).To(Equal(""))
			Expect(cm.StatsCallCount()).To(Equal(1))
		})
	})

	Context("stats fails", func() {
		BeforeEach(func() {
			cm.StatsReturns(container.Statistics{}, errors.New("stats failed"))
		})

		It("returns an error", func() {
			err := r.Events(containerId, nil, true)
			Expect(err).To(MatchError("stats failed"))
		})
	})
})
