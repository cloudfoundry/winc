package runtime_test

import (
	"errors"

	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/runtime"
	"code.cloudfoundry.org/winc/runtime/fakes"
	"code.cloudfoundry.org/winc/runtime/winsyscall"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("State", func() {
	const (
		bundlePath  = "some/dir"
		rootDir     = "dir-for-state-and-things"
		containerId = "container-for-state"
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

	Context("state succeeds", func() {
		var expectedJSON string

		BeforeEach(func() {
			sm.StateReturns(&specs.State{ID: containerId}, nil)
			expectedJSON = `{
  "ociVersion": "",
  "id": "container-for-state",
  "status": "",
  "bundle": ""
}`
		})

		It("writes the state to output", func() {
			Expect(r.State(containerId, output)).To(Succeed())

			_, c, wc, id, rd := stateFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(*wc).To(Equal(winsyscall.WinSyscall{}))
			Expect(id).To(Equal(containerId))
			Expect(rd).To(Equal(rootDir))

			Expect(string(output.Contents())).To(Equal(expectedJSON))
		})
	})

	Context("state fails", func() {
		BeforeEach(func() {
			sm.StateReturns(&specs.State{}, errors.New("couldn't get state"))
		})

		It("returns an error", func() {
			err := r.State(containerId, output)
			Expect(err).To(MatchError("couldn't get state"))
		})
	})

	Context("provided output is nil", func() {
		It("returns an error", func() {
			err := r.State(containerId, nil)
			Expect(err).To(MatchError("provided output is nil"))
		})
	})
})
