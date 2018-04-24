package container_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/fakes"
	"code.cloudfoundry.org/winc/hcs"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Start", func() {
	var (
		hcsClient        *fakes.HCSClient
		mounter          *fakes.Mounter
		processClient    *fakes.ProcessClient
		containerManager *container.Manager
		fakeContainer    *hcsfakes.Container
		rootDir          string
		bundlePath       string
		containerId      string
	)

	BeforeEach(func() {
		var err error

		rootDir, err = ioutil.TempDir("", "start.root")
		Expect(err).ToNot(HaveOccurred())

		stateDir := filepath.Join(rootDir, containerId)
		Expect(os.MkdirAll(stateDir, 0755)).To(Succeed())

		bundlePath, err = ioutil.TempDir("", "start.bundle")
		Expect(err).ToNot(HaveOccurred())

		containerId = filepath.Base(bundlePath)

		state := container.State{Bundle: bundlePath}
		contents, err := json.Marshal(state)
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(filepath.Join(stateDir, "state.json"), contents, 0644)).To(Succeed())

		hcsClient = &fakes.HCSClient{}
		mounter = &fakes.Mounter{}
		processClient = &fakes.ProcessClient{}
		logger := (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "state")

		containerManager = container.NewManager(logger, hcsClient, mounter, processClient, containerId, rootDir)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(rootDir)).To(Succeed())
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Context("when the container doesn't exist", func() {
		var missingContainerError = errors.New("container does not exist")

		BeforeEach(func() {
			hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{}, missingContainerError)
		})

		It("errors", func() {
			_, err := containerManager.Start(true, "")
			Expect(err).To(Equal(missingContainerError))
		})
	})

	Context("when the container exists", func() {
		var (
			fakeProcess           *hcsfakes.Process
			expectedProcessConfig *hcsshim.ProcessConfig
		)

		BeforeEach(func() {
			spec := &specs.Spec{
				Version: specs.Version,
				Process: &specs.Process{
					Args: []string{"powershell.exe", "Write-Host 'hi'"},
					Cwd:  "C:\\",
					User: specs.User{
						Username: "someuser",
					},
				},
				Root: &specs.Root{
					Path: "some-rootfs-path",
				},
				Windows: &specs.Windows{
					LayerFolders: []string{"some-layer-id"},
				},
				Hostname: "some-hostname",
			}
			writeSpec(bundlePath, spec)

			fakeContainer = &hcsfakes.Container{}
			fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{
				{ProcessId: 666, ImageName: "wininit.exe"},
				{ProcessId: 100, ImageName: "powershell.exe"},
			}, nil)
			fakeProcess = &hcsfakes.Process{}
			fakeProcess.PidReturns(100)
			fakeContainer.CreateProcessReturnsOnCall(0, fakeProcess, nil)
			processClient.StartTimeReturns(syscall.Filetime{LowDateTime: 100, HighDateTime: 200}, nil)
			hcsClient.OpenContainerReturns(fakeContainer, nil)
			hcsClient.GetContainerPropertiesReturnsOnCall(0, hcsshim.ContainerProperties{}, &hcs.NotFoundError{})
			hcsClient.GetContainerPropertiesReturnsOnCall(1, hcsshim.ContainerProperties{}, nil)
			hcsClient.CreateContainerReturns(fakeContainer, nil)
			_, err := containerManager.Create(bundlePath)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when detach is true", func() {
			BeforeEach(func() {
				expectedProcessConfig = &hcsshim.ProcessConfig{
					CommandLine:      `powershell.exe "Write-Host 'hi'"`,
					WorkingDirectory: "C:\\",
					User:             "someuser",
					Environment:      map[string]string{},
					CreateStdInPipe:  false,
					CreateStdOutPipe: false,
					CreateStdErrPipe: false,
				}
			})

			It("runs the user process", func() {
				proc, err := containerManager.Start(true, "")
				Expect(proc).To(Equal(fakeProcess))
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessCallCount()).To(Equal(1))
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
			})
			Context("the container has been stopped", func() {
				BeforeEach(func() {
					hcsClient.GetContainerPropertiesReturnsOnCall(1, hcsshim.ContainerProperties{Stopped: true}, nil)
				})

				It("errors and does not start the user process", func() {
					_, err := containerManager.Start(true, "")
					Expect(err).To(MatchError("cannot start a container in the stopped state"))
					Expect(fakeContainer.CreateProcessCallCount()).To(Equal(0))
				})
			})

			Context("the user process is running", func() {
				BeforeEach(func() {
					_, err := containerManager.Start(true, "")
					Expect(err).ToNot(HaveOccurred())
				})

				It("errors and does not start the user process", func() {
					_, err := containerManager.Start(true, "")
					Expect(err).To(MatchError("cannot start a container in the running state"))
					Expect(fakeContainer.CreateProcessCallCount()).To(Equal(1))
				})
			})

			Context("the user process has exited", func() {
				BeforeEach(func() {
					_, err := containerManager.Start(true, "")
					Expect(err).ToNot(HaveOccurred())
					fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{
						{ProcessId: 666, ImageName: "wininit.exe"},
					}, nil)
				})

				It("errors and does not start the user process", func() {
					_, err := containerManager.Start(true, "")
					Expect(err).To(MatchError("cannot start a container in the exited state"))
					Expect(fakeContainer.CreateProcessCallCount()).To(Equal(1))
				})
			})

			Context("exec of the user process fails", func() {
				BeforeEach(func() {
					fakeContainer.CreateProcessReturnsOnCall(0, nil, errors.New("couldn't exec process"))
				})

				It("returns an error", func() {
					_, err := containerManager.Start(true, "")
					Expect(pkgerrors.Cause(err)).To(BeAssignableToTypeOf(&container.CouldNotCreateProcessError{}))
				})

				It("sets the state to 'exited'", func() {
					containerManager.Start(true, "")
					s, err := containerManager.State()
					Expect(err).NotTo(HaveOccurred())
					Expect(s.Status).To(Equal("exited"))
				})
			})

			Context("getting the user start time fails", func() {
				BeforeEach(func() {
					processClient.StartTimeReturnsOnCall(0, syscall.Filetime{}, errors.New("blue screen"))
				})

				It("returns an error", func() {
					_, err := containerManager.Start(true, "")
					Expect(err).To(MatchError("blue screen"))
				})

				It("sets the state to 'exited'", func() {
					containerManager.Start(true, "")
					s, err := containerManager.State()
					Expect(err).NotTo(HaveOccurred())
					Expect(s.Status).To(Equal("exited"))
				})
			})
		})

		Context("when detach is false", func() {
			BeforeEach(func() {
				expectedProcessConfig = &hcsshim.ProcessConfig{
					CommandLine:      `powershell.exe "Write-Host 'hi'"`,
					WorkingDirectory: "C:\\",
					User:             "someuser",
					Environment:      map[string]string{},
					CreateStdInPipe:  true,
					CreateStdOutPipe: true,
					CreateStdErrPipe: true,
				}
			})

			It("runs the user process with i/o pipes", func() {
				proc, err := containerManager.Start(false, "")
				Expect(proc).To(Equal(fakeProcess))
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessCallCount()).To(Equal(1))
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))
			})
		})
	})
})
