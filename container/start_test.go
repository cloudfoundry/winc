package container_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

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
		containerManager *container.Manager
		fakeContainer    *hcsfakes.Container
		bundlePath       string
		containerId      string
	)

	BeforeEach(func() {
		var err error

		bundlePath, err = ioutil.TempDir("", "start.bundle")
		Expect(err).ToNot(HaveOccurred())

		containerId = filepath.Base(bundlePath)

		hcsClient = &fakes.HCSClient{}
		mounter = &fakes.Mounter{}
		logger := (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "state")

		containerManager = container.NewManager(logger, hcsClient, mounter, containerId)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Context("when the container doesn't exist", func() {
		var missingContainerError = errors.New("container does not exist")

		BeforeEach(func() {
			hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{}, missingContainerError)
		})

		It("errors", func() {
			_, err := containerManager.Start(bundlePath, true)
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
			hcsClient.OpenContainerReturns(fakeContainer, nil)
			hcsClient.GetContainerPropertiesReturnsOnCall(0, hcsshim.ContainerProperties{}, &hcs.NotFoundError{})
			hcsClient.GetContainerPropertiesReturnsOnCall(1, hcsshim.ContainerProperties{}, nil)
			hcsClient.CreateContainerReturns(fakeContainer, nil)
			_, err := containerManager.Create(bundlePath)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when attach is false", func() {
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

			It("runs the user process and mounts the volume", func() {
				proc, err := containerManager.Start(bundlePath, false)
				Expect(proc).To(Equal(fakeProcess))
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessCallCount()).To(Equal(1))
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))

				Expect(mounter.MountCallCount()).To(Equal(1))
				actualPid, actualVolumePath := mounter.MountArgsForCall(0)
				Expect(actualPid).To(Equal(100))
				Expect(actualVolumePath).To(Equal("some-rootfs-path"))
			})

			Context("exec of the user process fails", func() {
				BeforeEach(func() {
					fakeContainer.CreateProcessReturnsOnCall(0, nil, errors.New("couldn't exec process"))
				})

				It("returns an error", func() {
					_, err := containerManager.Start(bundlePath, true)
					Expect(pkgerrors.Cause(err)).To(BeAssignableToTypeOf(&container.CouldNotCreateProcessError{}))
				})
			})
		})

		Context("when attach is true", func() {
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

			It("runs the user process with i/o pipes and mounts the volume", func() {
				proc, err := containerManager.Start(bundlePath, true)
				Expect(proc).To(Equal(fakeProcess))
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeContainer.CreateProcessCallCount()).To(Equal(1))
				Expect(fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))

				Expect(mounter.MountCallCount()).To(Equal(1))
				actualPid, actualVolumePath := mounter.MountArgsForCall(0)
				Expect(actualPid).To(Equal(100))
				Expect(actualVolumePath).To(Equal("some-rootfs-path"))
			})
		})

		Context("when mounting the sandbox.vhdx fails", func() {
			BeforeEach(func() {
				mounter.MountReturns(errors.New("couldn't mount"))
				hcsClient.GetContainerPropertiesReturnsOnCall(1, hcsshim.ContainerProperties{Stopped: false}, nil)
			})

			It("returns an error", func() {
				_, err := containerManager.Start(bundlePath, true)
				Expect(err).To(MatchError("couldn't mount"))
			})
		})
	})
})
