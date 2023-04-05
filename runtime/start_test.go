package runtime_test

import (
	"code.cloudfoundry.org/winc/hcs"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"code.cloudfoundry.org/winc/runtime"
	"code.cloudfoundry.org/winc/runtime/container"
	"code.cloudfoundry.org/winc/runtime/fakes"
	"code.cloudfoundry.org/winc/runtime/winsyscall"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

var _ = Describe("Start", func() {
	const (
		bundlePath  = "some/dir"
		rootDir     = "dir-for-state-and-things"
		containerId = "container-for-exec"
		pidFile     = "something.pid"
	)
	var (
		mounter            *fakes.Mounter
		stateFactory       *fakes.StateFactory
		sm                 *fakes.StateManager
		containerFactory   *fakes.ContainerFactory
		cm                 *fakes.ContainerManager
		processWrapper     *fakes.ProcessWrapper
		wrappedProcess     *fakes.WrappedProcess
		unwrappedProcess   *hcsfakes.Process
		hcsQuery           *fakes.HCSQuery
		credentialSpecPath string
		r                  *runtime.Runtime
		spec               *specs.Spec
	)

	BeforeEach(func() {
		mounter = &fakes.Mounter{}
		hcsQuery = &fakes.HCSQuery{}
		stateFactory = &fakes.StateFactory{}
		sm = &fakes.StateManager{}
		containerFactory = &fakes.ContainerFactory{}
		cm = &fakes.ContainerManager{}
		processWrapper = &fakes.ProcessWrapper{}
		wrappedProcess = &fakes.WrappedProcess{}
		unwrappedProcess = &hcsfakes.Process{}
		spec = &specs.Spec{}

		spec.Process = &specs.Process{
			Cwd:  "C:\\Windows",
			Args: []string{"my", "process"},
		}
		spec.Root = &specs.Root{
			Path: "/some/path",
		}

		stateFactory.NewManagerReturns(sm)
		containerFactory.NewManagerReturns(cm)

		r = runtime.New(stateFactory, containerFactory, mounter, hcsQuery, processWrapper, rootDir, credentialSpecPath)
	})

	Context("starting the container succeeds", func() {
		BeforeEach(func() {
			state := &specs.State{Status: "created", Bundle: bundlePath}
			sm.StateReturns(state, nil)

			cm.SpecReturns(spec, nil)

			cm.ExecReturns(unwrappedProcess, nil)

			unwrappedProcess.PidReturns(99)

			processWrapper.WrapReturns(wrappedProcess)
		})

		It("gets the state, loads the bundle, execs the init process, sets the state, mounts the volume, and writes the pid file", func() {
			Expect(r.Start(containerId, pidFile)).To(Succeed())

			_, c, id := containerFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(id).To(Equal(containerId))

			_, c, wc, id, rd := stateFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(*wc).To(Equal(winsyscall.WinSyscall{}))
			Expect(id).To(Equal(containerId))
			Expect(rd).To(Equal(rootDir))

			Expect(sm.StateCallCount()).To(Equal(1))

			Expect(cm.SpecArgsForCall(0)).To(Equal(bundlePath))

			p, attach := cm.ExecArgsForCall(0)
			Expect(p).To(Equal(spec.Process))
			Expect(attach).To(BeTrue())

			Expect(unwrappedProcess.CloseCallCount()).To(Equal(1))

			Expect(sm.SetSuccessArgsForCall(0)).To(Equal(unwrappedProcess))

			pid, path, _ := mounter.MountArgsForCall(0)
			Expect(pid).To(Equal(99))
			Expect(path).To(Equal("/some/path"))

			Expect(processWrapper.WrapArgsForCall(0)).To(Equal(unwrappedProcess))

			Expect(wrappedProcess.WritePIDFileArgsForCall(0)).To(Equal(pidFile))
		})
	})

	Context("the state of the container is not 'created'", func() {
		BeforeEach(func() {
			state := &specs.State{Status: "running", Bundle: bundlePath}
			sm.StateReturns(state, nil)
		})

		It("returns an error", func() {
			err := r.Start(containerId, pidFile)
			Expect(err).To(MatchError("cannot start a container in the running state"))
		})
	})

	Context("starting the process fails due to a CouldNotCreateProcessError", func() {
		BeforeEach(func() {
			state := &specs.State{Status: "created", Bundle: bundlePath}
			sm.StateReturns(state, nil)

			cm.SpecReturns(spec, nil)

			e := errors.Wrap(&container.CouldNotCreateProcessError{}, "exec failed")
			cm.ExecReturns(nil, e)
		})

		It("returns an error and sets the state to failed", func() {
			err := r.Start(containerId, pidFile)
			Expect(err.Error()).To(ContainSubstring("could not start command"))
			Expect(sm.SetFailureCallCount()).To(Equal(1))
		})
	})

	Context("starting the process fails due to an unknown error type", func() {
		BeforeEach(func() {
			state := &specs.State{Status: "created", Bundle: bundlePath}
			sm.StateReturns(state, nil)

			cm.SpecReturns(spec, nil)

			cm.ExecReturns(nil, errors.New("couldn't exec"))
		})

		It("returns an error and doesn't update the state", func() {
			err := r.Start(containerId, pidFile)
			Expect(err).To(MatchError("couldn't exec"))
			Expect(sm.SetFailureCallCount()).To(Equal(0))
		})
	})

	Context("getting container state fails", func() {
		BeforeEach(func() {
			sm.StateReturns(nil, errors.New("couldn't get state"))
		})

		It("returns an error", func() {
			err := r.Start(containerId, pidFile)
			Expect(err).To(MatchError("couldn't get state"))
		})
	})

	Context("loading the bundle fails", func() {
		BeforeEach(func() {
			state := &specs.State{Status: "created", Bundle: bundlePath}
			sm.StateReturns(state, nil)

			cm.SpecReturns(nil, errors.New("couldn't load spec"))
		})

		It("returns an error", func() {
			err := r.Start(containerId, pidFile)
			Expect(err).To(MatchError("couldn't load spec"))
		})
	})

	Context("updating the state after start fails", func() {
		BeforeEach(func() {
			state := &specs.State{Status: "created", Bundle: bundlePath}
			sm.StateReturns(state, nil)

			cm.SpecReturns(spec, nil)

			cm.ExecReturns(unwrappedProcess, nil)
			sm.SetSuccessReturns(errors.New("updating state failed"))
		})

		It("returns an error", func() {
			err := r.Start(containerId, pidFile)
			Expect(err).To(MatchError("updating state failed"))
		})
	})

	Context("mounting the volume fails", func() {
		BeforeEach(func() {
			state := &specs.State{Status: "created", Bundle: bundlePath}
			sm.StateReturns(state, nil)

			cm.SpecReturns(spec, nil)

			cm.ExecReturns(unwrappedProcess, nil)
			mounter.MountReturns(errors.New("couldn't mount volume"))
		})

		It("returns an error", func() {
			err := r.Start(containerId, pidFile)
			Expect(err).To(MatchError("couldn't mount volume"))
		})
	})

	Context("writing the pid file fails", func() {
		BeforeEach(func() {
			state := &specs.State{Status: "created", Bundle: bundlePath}
			sm.StateReturns(state, nil)

			cm.SpecReturns(spec, nil)

			cm.ExecReturns(unwrappedProcess, nil)

			unwrappedProcess.PidReturns(99)

			processWrapper.WrapReturns(wrappedProcess)
			wrappedProcess.WritePIDFileReturns(errors.New("couldn't write pidfile"))
		})

		It("returns an error", func() {
			err := r.Start(containerId, pidFile)
			Expect(err).To(MatchError("couldn't write pidfile"))
		})
	})
})
