package runtime_test

import (
	"github.com/pkg/errors"

	"code.cloudfoundry.org/winc/hcs"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"code.cloudfoundry.org/winc/runtime"
	"code.cloudfoundry.org/winc/runtime/fakes"
	"code.cloudfoundry.org/winc/runtime/winsyscall"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Delete", func() {
	const (
		bundlePath  = "some/dir"
		rootDir     = "dir-for-state-and-things"
		containerId = "container-for-state"
		pidFile     = "something.pid"
	)
	var (
		mounter          *fakes.Mounter
		stateFactory     *fakes.StateFactory
		sm               *fakes.StateManager
		containerFactory *fakes.ContainerFactory
		cm               *fakes.ContainerManager
		processWrapper   *fakes.ProcessWrapper
		wrappedProcess   *fakes.WrappedProcess
		unwrappedProcess *hcsfakes.Process
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
		wrappedProcess = &fakes.WrappedProcess{}
		unwrappedProcess = &hcsfakes.Process{}
		spec = &specs.Spec{}

		stateFactory.NewManagerReturns(sm)
		containerFactory.NewManagerReturns(cm)

		r = runtime.New(stateFactory, containerFactory, mounter, hcsQuery, processWrapper, rootDir)
	})

	BeforeEach(func() {
		state := &specs.State{Status: "stopped", Bundle: bundlePath, Pid: 99}
		sm.StateReturns(state, nil)
	})

	It("unmounts the volume, deletes the state and deletes the container", func() {
		Expect(r.Delete(containerId, true)).To(Succeed())

		_, c, id := containerFactory.NewManagerArgsForCall(0)
		Expect(*c).To(Equal(hcs.Client{}))
		Expect(id).To(Equal(containerId))

		_, c, wc, id, rd := stateFactory.NewManagerArgsForCall(0)
		Expect(*c).To(Equal(hcs.Client{}))
		Expect(*wc).To(Equal(winsyscall.WinSyscall{}))
		Expect(id).To(Equal(containerId))
		Expect(rd).To(Equal(rootDir))

		Expect(mounter.UnmountArgsForCall(0)).To(Equal(99))
		Expect(sm.DeleteCallCount()).To(Equal(1))
		Expect(cm.DeleteArgsForCall(0)).To(BeTrue())
	})

	Context("getting state fails", func() {
		Context("force is true", func() {
			Context("the error is hcs.NotFoundError", func() {
				BeforeEach(func() {
					sm.StateReturns(nil, &hcs.NotFoundError{})
				})

				It("returns success", func() {
					Expect(r.Delete(containerId, true)).To(Succeed())

					Expect(mounter.UnmountCallCount()).To(Equal(0))
					Expect(sm.DeleteCallCount()).To(Equal(0))
					Expect(cm.DeleteCallCount()).To(Equal(0))
				})
			})

			Context("the error is of unknown type", func() {
				BeforeEach(func() {
					sm.StateReturns(nil, errors.New("couldn't get state"))
				})

				It("returns the error", func() {
					err := r.Delete(containerId, true)
					Expect(err).To(MatchError("couldn't get state"))

					Expect(mounter.UnmountCallCount()).To(Equal(0))
					Expect(sm.DeleteCallCount()).To(Equal(1))
					Expect(cm.DeleteArgsForCall(0)).To(BeTrue())
				})
			})
		})

		Context("force is false", func() {
			Context("the error is hcs.NotFoundError", func() {
				BeforeEach(func() {
					sm.StateReturns(nil, &hcs.NotFoundError{})
				})

				It("returns the error", func() {
					err := r.Delete(containerId, false)
					Expect(err).To(MatchError(&hcs.NotFoundError{}))

					Expect(mounter.UnmountCallCount()).To(Equal(0))
					Expect(sm.DeleteCallCount()).To(Equal(0))
					Expect(cm.DeleteCallCount()).To(Equal(0))
				})
			})

			Context("the error is of unknown type", func() {
				BeforeEach(func() {
					sm.StateReturns(nil, errors.New("couldn't get state"))
				})

				It("returns the error", func() {
					err := r.Delete(containerId, false)
					Expect(err).To(MatchError("couldn't get state"))

					Expect(mounter.UnmountCallCount()).To(Equal(0))
					Expect(sm.DeleteCallCount()).To(Equal(1))
					Expect(cm.DeleteArgsForCall(0)).To(BeFalse())
				})
			})
		})
	})

	Context("the state doesn't have a pid", func() {
		BeforeEach(func() {
			sm.StateReturns(&specs.State{}, nil)
		})

		It("deletes the state and deletes the container", func() {
			Expect(r.Delete(containerId, true)).To(Succeed())

			Expect(mounter.UnmountCallCount()).To(Equal(0))
			Expect(sm.DeleteCallCount()).To(Equal(1))
			Expect(cm.DeleteArgsForCall(0)).To(BeTrue())
		})
	})

	Context("unmounting fails", func() {
		BeforeEach(func() {
			mounter.UnmountReturns(errors.New("couldn't unmount"))
		})

		It("deletes the state and deletes the container", func() {
			err := r.Delete(containerId, true)
			Expect(err).To(MatchError("couldn't unmount"))

			Expect(mounter.UnmountCallCount()).To(Equal(1))
			Expect(sm.DeleteCallCount()).To(Equal(1))
			Expect(cm.DeleteArgsForCall(0)).To(BeTrue())
		})
	})

	Context("deleting state fails", func() {
		BeforeEach(func() {
			sm.DeleteReturns(errors.New("couldn't delete state"))
		})

		It("deletes the container", func() {
			err := r.Delete(containerId, true)
			Expect(err).To(MatchError("couldn't delete state"))

			Expect(mounter.UnmountCallCount()).To(Equal(1))
			Expect(sm.DeleteCallCount()).To(Equal(1))
			Expect(cm.DeleteArgsForCall(0)).To(BeTrue())
		})
	})

	Context("deleting the container fails", func() {
		BeforeEach(func() {
			cm.DeleteReturns(errors.New("couldn't delete container"))
		})

		It("returns an error", func() {
			err := r.Delete(containerId, true)
			Expect(err).To(MatchError("couldn't delete container"))

			Expect(mounter.UnmountCallCount()).To(Equal(1))
			Expect(sm.DeleteCallCount()).To(Equal(1))
			Expect(cm.DeleteArgsForCall(0)).To(BeTrue())
		})
	})
})
