package runtime_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/hcs"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"code.cloudfoundry.org/winc/runtime"
	"code.cloudfoundry.org/winc/runtime/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Exec", func() {
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
		processSpecDir     string
		processSpecFile    string
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

		stateFactory.NewManagerReturns(sm)
		containerFactory.NewManagerReturns(cm)

		var err error
		processSpecDir, err = ioutil.TempDir("", "runtime.exec")
		Expect(err).NotTo(HaveOccurred())
		processSpecFile = filepath.Join(processSpecDir, "process.json")

		r = runtime.New(stateFactory, containerFactory, mounter, hcsQuery, processWrapper, rootDir, credentialSpecPath)

		processSpec := specs.Process{
			User: specs.User{Username: "some-user"},
			Cwd:  "c:\\windows",
			Args: []string{"my", "program"},
			Env:  []string{"FOO=bar"},
		}

		c, err := json.Marshal(processSpec)
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(processSpecFile, c, 0644)).To(Succeed())

		unwrappedProcess = &hcsfakes.Process{}

		stdin = gbytes.NewBuffer()
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		io = runtime.IO{Stdin: stdin, Stdout: stdout, Stderr: stderr}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(processSpecDir)).To(Succeed())
	})

	Context("detach is true", func() {
		BeforeEach(func() {
			cm.ExecReturns(unwrappedProcess, nil)
			processWrapper.WrapReturns(wrappedProcess)
		})

		It("loads the process config, execs the process, and writes the pidfile", func() {
			exitCode, err := r.Exec(containerId, processSpecFile, pidFile, nil, io, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			_, c, id := containerFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(id).To(Equal(containerId))

			spec, attach := cm.ExecArgsForCall(0)
			Expect(*spec).To(Equal(specs.Process{
				User: specs.User{Username: "some-user"},
				Cwd:  "c:\\windows",
				Args: []string{"my", "program"},
				Env:  []string{"FOO=bar"},
			}))
			Expect(attach).To(BeFalse())

			Expect(unwrappedProcess.CloseCallCount()).To(Equal(1))

			Expect(processWrapper.WrapArgsForCall(0)).To(Equal(unwrappedProcess))

			Expect(wrappedProcess.WritePIDFileArgsForCall(0)).To(Equal(pidFile))
			Expect(wrappedProcess.SetInterruptCallCount()).To(Equal(0))
			Expect(wrappedProcess.AttachIOCallCount()).To(Equal(0))
		})
	})

	Context("process spec overrides are passed", func() {
		BeforeEach(func() {
			cm.ExecReturns(unwrappedProcess, nil)
			processWrapper.WrapReturns(wrappedProcess)
		})

		It("uses the values from the overrides", func() {
			overrides := specs.Process{
				Cwd: "c:\\some-other-dir",
			}
			exitCode, err := r.Exec(containerId, processSpecFile, pidFile, &overrides, io, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			_, c, id := containerFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(id).To(Equal(containerId))

			spec, _ := cm.ExecArgsForCall(0)
			Expect(*spec).To(Equal(specs.Process{
				User: specs.User{Username: "some-user"},
				Cwd:  "c:\\some-other-dir",
				Args: []string{"my", "program"},
				Env:  []string{"FOO=bar"},
			}))
		})
	})

	Context("the process spec is invalid", func() {
		BeforeEach(func() {
			processSpec := specs.Process{
				User: specs.User{Username: "some-user"},
				Cwd:  "c:\\windows",
				Env:  []string{"FOO=bar"},
			}

			c, err := json.Marshal(processSpec)
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(processSpecFile, c, 0644)).To(Succeed())
		})

		It("returns an error", func() {
			exitCode, err := r.Exec(containerId, processSpecFile, pidFile, nil, io, true)
			Expect(err).To(HaveOccurred())
			Expect(exitCode).To(Equal(1))
			Expect(err.Error()).To(ContainSubstring("args must not be empty"))
		})
	})

	Context("detach is false", func() {
		BeforeEach(func() {
			cm.ExecReturns(unwrappedProcess, nil)
			processWrapper.WrapReturns(wrappedProcess)
			wrappedProcess.AttachIOReturns(9, nil)
		})

		It("execs the process and waits for it", func() {
			exitCode, err := r.Exec(containerId, processSpecFile, pidFile, nil, io, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(9))

			_, c, id := containerFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(id).To(Equal(containerId))

			spec, attach := cm.ExecArgsForCall(0)
			Expect(*spec).To(Equal(specs.Process{
				User: specs.User{Username: "some-user"},
				Cwd:  "c:\\windows",
				Args: []string{"my", "program"},
				Env:  []string{"FOO=bar"},
			}))
			Expect(attach).To(BeTrue())

			Expect(unwrappedProcess.CloseCallCount()).To(Equal(1))

			Expect(processWrapper.WrapArgsForCall(0)).To(Equal(unwrappedProcess))

			Expect(wrappedProcess.WritePIDFileArgsForCall(0)).To(Equal(pidFile))

			s := make(chan os.Signal, 1)
			sigChan := wrappedProcess.SetInterruptArgsForCall(0)

			Expect(sigChan).To(BeAssignableToTypeOf(s))

			si, so, se := wrappedProcess.AttachIOArgsForCall(0)
			Expect(si).To(Equal(stdin))
			Expect(so).To(Equal(stdout))
			Expect(se).To(Equal(stderr))
		})

		Context("attaching io fails", func() {
			BeforeEach(func() {
				cm.ExecReturns(unwrappedProcess, nil)
				processWrapper.WrapReturns(wrappedProcess)
				wrappedProcess.AttachIOReturns(-1, errors.New("couldn't attach"))
			})

			It("returns an error", func() {
				exitCode, err := r.Exec(containerId, processSpecFile, pidFile, nil, io, false)
				Expect(err).To(HaveOccurred())
				Expect(exitCode).To(Equal(-1))
				Expect(err).To(MatchError("couldn't attach"))
			})
		})
	})

	Context("exec fails", func() {
		BeforeEach(func() {
			cm.ExecReturns(nil, errors.New("couldn't exec"))
		})

		It("returns an error", func() {
			exitCode, err := r.Exec(containerId, processSpecFile, pidFile, nil, io, false)
			Expect(err).To(HaveOccurred())
			Expect(exitCode).To(Equal(1))
			Expect(err).To(MatchError("couldn't exec"))
		})
	})

	Context("writing the pid file fails", func() {
		BeforeEach(func() {
			cm.ExecReturns(unwrappedProcess, nil)
			processWrapper.WrapReturns(wrappedProcess)
			wrappedProcess.WritePIDFileReturns(errors.New("couldn't write pidfile"))
		})

		It("returns an error", func() {
			exitCode, err := r.Exec(containerId, processSpecFile, pidFile, nil, io, false)
			Expect(err).To(HaveOccurred())
			Expect(exitCode).To(Equal(1))
			Expect(err).To(MatchError("couldn't write pidfile"))
		})
	})
})
