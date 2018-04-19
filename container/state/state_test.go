package state_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/winc/container/state"
	"code.cloudfoundry.org/winc/container/state/fakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("State", func() {
	const (
		containerId = "some-container"
		bundlePath  = "some/path/some-container"
	)

	var (
		hcsClient *fakes.HCSClient
		sc        *fakes.WinSyscall
		sm        *state.Manager
		rootDir   string
		stateFile string
	)

	BeforeEach(func() {
		var err error
		rootDir, err = ioutil.TempDir("", "create.root")
		Expect(err).ToNot(HaveOccurred())
		stateFile = filepath.Join(rootDir, containerId, "state.json")

		hcsClient = &fakes.HCSClient{}
		sc = &fakes.WinSyscall{}
		sm = state.New(rootDir, containerId, hcsClient, sc)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(rootDir)).To(Succeed())
	})

	Describe("Initialize", func() {
		It("writes the bundle path to state.json in <rootDir>/<containerId>/", func() {
			Expect(sm.Initialize(bundlePath)).To(Succeed())

			var state state.State
			contents, err := ioutil.ReadFile(stateFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(json.Unmarshal(contents, &state)).To(Succeed())

			Expect(state.Bundle).To(Equal(bundlePath))
			Expect(state.PID).To(Equal(0))
			Expect(state.StartTime).To(Equal(syscall.Filetime{}))
			Expect(state.ExecFailed).To(Equal(false))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			Expect(sm.Initialize(bundlePath)).To(Succeed())
			Expect(stateFile).To(BeAnExistingFile())
		})

		It("removes the state dir, leaving the root dir", func() {
			Expect(sm.Delete()).To(Succeed())
			Expect(filepath.Dir(stateFile)).NotTo(BeADirectory())
			Expect(rootDir).To(BeADirectory())
		})
	})

	Describe("BundlePath", func() {
		BeforeEach(func() {
			Expect(sm.Initialize(bundlePath)).To(Succeed())
			Expect(stateFile).To(BeAnExistingFile())
		})

		It("returns the bundle path", func() {
			p, err := sm.BundlePath()
			Expect(err).NotTo(HaveOccurred())
			Expect(p).To(Equal(bundlePath))
		})
	})

	Describe("State", func() {
		var (
			s state.State
		)
		BeforeEach(func() {
			s = state.State{
				PID:        1234,
				Bundle:     bundlePath,
				StartTime:  syscall.Filetime{HighDateTime: 123, LowDateTime: 456},
				ExecFailed: false,
			}

			c, err := json.Marshal(s)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.MkdirAll(filepath.Dir(stateFile), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(stateFile, c, 0644)).To(Succeed())
		})

		It("includes the necessary fields in the oci state", func() {
			ociState, err := sm.State()
			Expect(err).NotTo(HaveOccurred())
			Expect(ociState.Bundle).To(Equal(bundlePath))
			Expect(ociState.Pid).To(Equal(1234))
			Expect(ociState.ID).To(Equal(containerId))
			Expect(ociState.Version).To(Equal(specs.Version))
		})

		Context("hcsshim reports the container as stopped", func() {
			BeforeEach(func() {
				hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{Stopped: true}, nil)
			})

			It("reports the container is stopped", func() {
				ociState, err := sm.State()
				Expect(err).NotTo(HaveOccurred())
				Expect(ociState.Status).To(Equal("stopped"))
			})
		})

		Context("state.json has no pid and no stop time", func() {
			BeforeEach(func() {
				s.PID = 0
				s.StartTime = syscall.Filetime{}
				c, err := json.Marshal(s)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.WriteFile(stateFile, c, 0644)).To(Succeed())
			})

			It("reports the container is created", func() {
				ociState, err := sm.State()
				Expect(err).NotTo(HaveOccurred())
				Expect(ociState.Status).To(Equal("created"))
			})
		})

		Context("container init process is running", func() {
			BeforeEach(func() {
				ph := syscall.Handle(0xf00d)
				sc.OpenProcessReturns(ph, nil)
				sc.GetProcessTimesStub = func(h syscall.Handle, creation *syscall.Filetime, _, _, _ *syscall.Filetime) error {
					Expect(h).To(Equal(ph))
					creation.HighDateTime = 123
					creation.LowDateTime = 456
					return nil
				}
			})

			It("reports the container is running", func() {
				ociState, err := sm.State()
				Expect(err).NotTo(HaveOccurred())

				flags, inherit, pid := sc.OpenProcessArgsForCall(0)
				Expect(flags).To(Equal(uint32(syscall.PROCESS_QUERY_INFORMATION)))
				Expect(inherit).To(Equal(false))
				Expect(pid).To(Equal(uint32(1234)))
				Expect(ociState.Status).To(Equal("running"))
			})
		})

		Context("no process with container init pid is running", func() {
			BeforeEach(func() {
				sc.OpenProcessReturns(0, syscall.Errno(0x57))
			})

			It("reports the container is stopped", func() {
				ociState, err := sm.State()
				Expect(err).NotTo(HaveOccurred())

				flags, inherit, pid := sc.OpenProcessArgsForCall(0)
				Expect(flags).To(Equal(uint32(syscall.PROCESS_QUERY_INFORMATION)))
				Expect(inherit).To(Equal(false))
				Expect(pid).To(Equal(uint32(1234)))
				Expect(ociState.Status).To(Equal("stopped"))
			})
		})

		Context("a process with container init pid is running, but with a different start time", func() {
			BeforeEach(func() {
				ph := syscall.Handle(0xf00d)
				sc.OpenProcessReturns(ph, nil)
				sc.GetProcessTimesStub = func(h syscall.Handle, creation *syscall.Filetime, _, _, _ *syscall.Filetime) error {
					Expect(h).To(Equal(ph))
					creation.HighDateTime = 123
					creation.LowDateTime = 789
					return nil
				}
			})

			It("reports the container is stopped", func() {
				ociState, err := sm.State()
				Expect(err).NotTo(HaveOccurred())

				flags, inherit, pid := sc.OpenProcessArgsForCall(0)
				Expect(flags).To(Equal(uint32(syscall.PROCESS_QUERY_INFORMATION)))
				Expect(inherit).To(Equal(false))
				Expect(pid).To(Equal(uint32(1234)))
				Expect(ociState.Status).To(Equal("stopped"))
			})
		})

		Context("exec of the init process failed", func() {
			BeforeEach(func() {
				s.ExecFailed = true
				c, err := json.Marshal(s)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.WriteFile(stateFile, c, 0644)).To(Succeed())
			})

			It("reports the container is stopped", func() {
				ociState, err := sm.State()
				Expect(err).NotTo(HaveOccurred())
				Expect(ociState.Status).To(Equal("stopped"))
			})
		})

		Context("getting container properties fails", func() {
			BeforeEach(func() {
				hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{}, errors.New("hcsshim failed"))
			})

			It("wraps the error", func() {
				_, err := sm.State()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("GetContainerProperties: hcsshim failed"))
			})
		})

		Context("openprocess fails", func() {
			BeforeEach(func() {
				sc.OpenProcessReturns(0, syscall.Errno(0x5))
			})

			It("wraps the error", func() {
				_, err := sm.State()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("OpenProcess: Access is denied."))
			})
		})

		Context("getprocesstimes fails", func() {
			BeforeEach(func() {
				sc.GetProcessTimesReturns(syscall.Errno(0x6))
			})

			It("wraps the error", func() {
				_, err := sm.State()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("GetProcessTimes: The handle is invalid."))
			})
		})

		Context("pid is set in state.json but no start time is set", func() {
			BeforeEach(func() {
				s.StartTime = syscall.Filetime{}
				c, err := json.Marshal(s)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.WriteFile(stateFile, c, 0644)).To(Succeed())
			})

			It("returns an invalid state error", func() {
				_, err := sm.State()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("invalid state: PID 1234, start time {LowDateTime:0 HighDateTime:0}"))
			})
		})

		Context("start time is set in state.json but no pid is set", func() {
			BeforeEach(func() {
				s.PID = 0
				c, err := json.Marshal(s)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.WriteFile(stateFile, c, 0644)).To(Succeed())
			})

			It("returns an invalid state error", func() {
				_, err := sm.State()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("invalid state: PID 0, start time {LowDateTime:456 HighDateTime:123}"))
			})
		})
	})
})
