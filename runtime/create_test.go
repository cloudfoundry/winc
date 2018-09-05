package runtime_test

import (
	"errors"

	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/runtime"
	"code.cloudfoundry.org/winc/runtime/fakes"
	"code.cloudfoundry.org/winc/runtime/winsyscall"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Create", func() {
	const (
		bundlePath  = "some/dir"
		rootDir     = "dir-for-state-and-things"
		containerId = "container-to-create"
	)
	var (
		mounter          *fakes.Mounter
		stateFactory     *fakes.StateFactory
		sm               *fakes.StateManager
		containerFactory *fakes.ContainerFactory
		cm               *fakes.ContainerManager
		processWrapper   *fakes.ProcessWrapper
		hcsQuery         *fakes.HCSQuery
		r                *runtime.Runtime
		spec             *specs.Spec
	)

	BeforeEach(func() {
		mounter = &fakes.Mounter{}
		hcsQuery = &fakes.HCSQuery{}
		stateFactory = &fakes.StateFactory{}
		sm = &fakes.StateManager{}
		containerFactory = &fakes.ContainerFactory{}
		cm = &fakes.ContainerManager{}
		processWrapper = &fakes.ProcessWrapper{}
		spec = &specs.Spec{}

		stateFactory.NewManagerReturns(sm)
		containerFactory.NewManagerReturns(cm)

		r = runtime.New(stateFactory, containerFactory, mounter, hcsQuery, processWrapper, rootDir)
	})

	Context("success", func() {
		BeforeEach(func() {
			cm.SpecReturns(spec, nil)
		})

		It("loads the spec, creates the container, and intializes the state", func() {
			Expect(r.Create(containerId, bundlePath)).To(Succeed())

			_, c, id := containerFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(id).To(Equal(containerId))

			_, c, wc, id, rd := stateFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(*wc).To(Equal(winsyscall.WinSyscall{}))
			Expect(id).To(Equal(containerId))
			Expect(rd).To(Equal(rootDir))

			Expect(cm.SpecArgsForCall(0)).To(Equal(bundlePath))
			Expect(cm.CreateArgsForCall(0)).To(Equal(spec))
			Expect(sm.InitializeArgsForCall(0)).To(Equal(bundlePath))
		})
	})

	Context("loading the spec fails", func() {
		BeforeEach(func() {
			cm.SpecReturns(nil, errors.New("bad spec"))
		})

		It("returns the error", func() {
			err := r.Create(containerId, bundlePath)
			Expect(err).To(MatchError("bad spec"))
		})
	})

	Context("creating the container fails", func() {
		BeforeEach(func() {
			cm.CreateReturns(errors.New("hcsshim fell over"))
		})

		It("returns the error", func() {
			err := r.Create(containerId, bundlePath)
			Expect(err).To(MatchError("hcsshim fell over"))
		})
	})

	Context("initializing state fails", func() {
		BeforeEach(func() {
			cm.SpecReturns(spec, nil)
			sm.InitializeReturns(errors.New("state init failed"))
		})

		It("deletes the container", func() {
			err := r.Create(containerId, bundlePath)
			Expect(err).To(MatchError("state init failed"))

			Expect(cm.DeleteCallCount()).To(Equal(1))
			force := cm.DeleteArgsForCall(0)
			Expect(force).To(Equal(false))
		})
	})
})
