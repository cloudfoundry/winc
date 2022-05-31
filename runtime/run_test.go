package runtime_test

import (
	"os"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/winc/hcs"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"code.cloudfoundry.org/winc/runtime"
	"code.cloudfoundry.org/winc/runtime/container"
	"code.cloudfoundry.org/winc/runtime/fakes"
	"code.cloudfoundry.org/winc/runtime/winsyscall"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Run", func() {
	const (
		bundlePath  = "some/dir"
		rootDir     = "dir-for-state-and-things"
		containerId = "container-for-state"
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
		io                 runtime.IO
		stdin              *gbytes.Buffer
		stdout             *gbytes.Buffer
		stderr             *gbytes.Buffer
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

		stdin = gbytes.NewBuffer()
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		io = runtime.IO{Stdin: stdin, Stdout: stdout, Stderr: stderr}
	})

	Context("detach is true", func() {
		BeforeEach(func() {
			cm.SpecReturns(spec, nil)

			cm.ExecReturns(unwrappedProcess, nil)

			unwrappedProcess.PidReturns(99)

			processWrapper.WrapReturns(wrappedProcess)
		})

		It("creates the container and execs the init process", func() {
			exitCode, err := r.Run(containerId, bundlePath, pidFile, io, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

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

			p, attach := cm.ExecArgsForCall(0)
			Expect(p).To(Equal(spec.Process))
			Expect(attach).To(BeFalse())

			Expect(unwrappedProcess.CloseCallCount()).To(Equal(1))

			Expect(sm.SetSuccessArgsForCall(0)).To(Equal(unwrappedProcess))

			pid, path, _ := mounter.MountArgsForCall(0)
			Expect(pid).To(Equal(99))
			Expect(path).To(Equal("/some/path"))

			Expect(processWrapper.WrapArgsForCall(0)).To(Equal(unwrappedProcess))

			Expect(wrappedProcess.WritePIDFileArgsForCall(0)).To(Equal(pidFile))
			Expect(wrappedProcess.SetInterruptCallCount()).To(Equal(0))
			Expect(wrappedProcess.AttachIOCallCount()).To(Equal(0))
		})
	})

	Context("detach is false", func() {
		BeforeEach(func() {
			cm.SpecReturns(spec, nil)

			cm.ExecReturns(unwrappedProcess, nil)

			unwrappedProcess.PidReturns(99)

			processWrapper.WrapReturns(wrappedProcess)
			wrappedProcess.AttachIOReturns(9, nil)

			state := &specs.State{Status: "stopped", Bundle: bundlePath, Pid: 99}
			sm.StateReturns(state, nil)
		})

		It("creates the container, execs the init process, waits for it, and deletes the container", func() {
			exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(9))

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

			s := make(chan os.Signal, 1)
			sigChan := wrappedProcess.SetInterruptArgsForCall(0)

			Expect(sigChan).To(BeAssignableToTypeOf(s))

			si, so, se := wrappedProcess.AttachIOArgsForCall(0)
			Expect(si).To(Equal(stdin))
			Expect(so).To(Equal(stdout))
			Expect(se).To(Equal(stderr))

			Expect(mounter.UnmountArgsForCall(0)).To(Equal(99))
			Expect(sm.DeleteCallCount()).To(Equal(1))
			Expect(cm.DeleteArgsForCall(0)).To(BeFalse())
		})

		Context("attaching io fails", func() {
			BeforeEach(func() {
				cm.ExecReturns(unwrappedProcess, nil)
				processWrapper.WrapReturns(wrappedProcess)
				wrappedProcess.AttachIOReturns(-1, errors.New("couldn't attach"))
			})

			It("unmounts the volume, deletes the state and deletes the container", func() {
				exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
				Expect(err).To(MatchError("couldn't attach"))
				Expect(exitCode).To(Equal(-1))

				Expect(mounter.UnmountArgsForCall(0)).To(Equal(99))
				Expect(sm.DeleteCallCount()).To(Equal(1))
				Expect(cm.DeleteArgsForCall(0)).To(BeFalse())
			})
		})

		Context("getting state fails", func() {
			BeforeEach(func() {
				sm.StateReturns(nil, errors.New("couldn't get state"))
			})

			It("deletes the state and deletes the container", func() {
				exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
				Expect(err).To(MatchError("couldn't get state"))
				Expect(exitCode).To(Equal(1))

				Expect(mounter.UnmountCallCount()).To(Equal(0))
				Expect(sm.DeleteCallCount()).To(Equal(1))
				Expect(cm.DeleteArgsForCall(0)).To(BeFalse())
			})
		})

		Context("the state doesn't have a pid", func() {
			BeforeEach(func() {
				sm.StateReturns(&specs.State{}, nil)
			})

			It("deletes the state and deletes the container", func() {
				exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCode).To(Equal(9))

				Expect(mounter.UnmountCallCount()).To(Equal(0))
				Expect(sm.DeleteCallCount()).To(Equal(1))
				Expect(cm.DeleteArgsForCall(0)).To(BeFalse())
			})
		})

		Context("unmounting fails", func() {
			BeforeEach(func() {
				mounter.UnmountReturns(errors.New("couldn't unmount"))
			})

			It("deletes the state and deletes the container", func() {
				exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
				Expect(err).To(MatchError("couldn't unmount"))
				Expect(exitCode).To(Equal(1))

				Expect(mounter.UnmountCallCount()).To(Equal(1))
				Expect(sm.DeleteCallCount()).To(Equal(1))
				Expect(cm.DeleteArgsForCall(0)).To(BeFalse())
			})
		})

		Context("deleting state fails", func() {
			BeforeEach(func() {
				sm.DeleteReturns(errors.New("couldn't delete state"))
			})

			It("deletes the container", func() {
				exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
				Expect(err).To(MatchError("couldn't delete state"))
				Expect(exitCode).To(Equal(1))

				Expect(mounter.UnmountCallCount()).To(Equal(1))
				Expect(sm.DeleteCallCount()).To(Equal(1))
				Expect(cm.DeleteArgsForCall(0)).To(BeFalse())
			})
		})

		Context("deleting the container fails", func() {
			BeforeEach(func() {
				cm.DeleteReturns(errors.New("couldn't delete container"))
			})

			It("deletes the container", func() {
				exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
				Expect(err).To(MatchError("couldn't delete container"))
				Expect(exitCode).To(Equal(1))

				Expect(mounter.UnmountCallCount()).To(Equal(1))
				Expect(sm.DeleteCallCount()).To(Equal(1))
				Expect(cm.DeleteArgsForCall(0)).To(BeFalse())
			})
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
			exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
			Expect(err).To(MatchError("hcsshim fell over"))
			Expect(exitCode).To(Equal(1))
		})
	})

	Context("initializing state fails", func() {
		BeforeEach(func() {
			cm.SpecReturns(spec, nil)
			sm.InitializeReturns(errors.New("state init failed"))
		})

		It("deletes the container", func() {
			exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
			Expect(err).To(MatchError("state init failed"))
			Expect(exitCode).To(Equal(1))

			Expect(cm.DeleteCallCount()).To(Equal(1))
			force := cm.DeleteArgsForCall(0)
			Expect(force).To(Equal(false))
		})
	})

	Context("starting the process fails due to a CouldNotCreateProcessError", func() {
		BeforeEach(func() {
			cm.SpecReturns(spec, nil)

			e := errors.Wrap(&container.CouldNotCreateProcessError{}, "exec failed")
			cm.ExecReturns(nil, e)
		})

		It("returns an error and sets the state to failed", func() {
			exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
			Expect(err.Error()).To(ContainSubstring("could not start command"))
			Expect(exitCode).To(Equal(1))
			Expect(sm.SetFailureCallCount()).To(Equal(1))
		})
	})

	Context("starting the process fails due to an unknown error type", func() {
		BeforeEach(func() {
			cm.SpecReturns(spec, nil)

			cm.ExecReturns(nil, errors.New("couldn't exec"))
		})

		It("returns an error and doesn't update the state", func() {
			exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
			Expect(err).To(MatchError("couldn't exec"))
			Expect(exitCode).To(Equal(1))
			Expect(sm.SetFailureCallCount()).To(Equal(0))
		})
	})

	Context("loading the bundle fails", func() {
		BeforeEach(func() {
			cm.SpecReturns(nil, errors.New("couldn't load spec"))
		})

		It("returns an error", func() {
			exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
			Expect(err).To(MatchError("couldn't load spec"))
			Expect(exitCode).To(Equal(1))
		})
	})

	Context("updating the state after start fails", func() {
		BeforeEach(func() {
			cm.SpecReturns(spec, nil)

			cm.ExecReturns(unwrappedProcess, nil)
			sm.SetSuccessReturns(errors.New("updating state failed"))
		})

		It("returns an error", func() {
			exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
			Expect(err).To(MatchError("updating state failed"))
			Expect(exitCode).To(Equal(1))
		})
	})

	Context("mounting the volume fails", func() {
		BeforeEach(func() {
			cm.SpecReturns(spec, nil)

			cm.ExecReturns(unwrappedProcess, nil)
			mounter.MountReturns(errors.New("couldn't mount volume"))
		})

		It("returns an error", func() {
			exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
			Expect(err).To(MatchError("couldn't mount volume"))
			Expect(exitCode).To(Equal(1))
		})
	})

	Context("writing the pid file fails", func() {
		BeforeEach(func() {
			cm.SpecReturns(spec, nil)

			cm.ExecReturns(unwrappedProcess, nil)

			unwrappedProcess.PidReturns(99)

			processWrapper.WrapReturns(wrappedProcess)
			wrappedProcess.WritePIDFileReturns(errors.New("couldn't write pidfile"))
		})

		It("returns an error", func() {
			exitCode, err := r.Run(containerId, bundlePath, pidFile, io, false)
			Expect(err).To(MatchError("couldn't write pidfile"))
			Expect(exitCode).To(Equal(1))
		})
	})
})
