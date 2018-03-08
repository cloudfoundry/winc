package container_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/fakes"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("State", func() {
	const (
		containerId = "some-id"
		bundlePath  = "some-bundle-dir"
	)

	var (
		hcsClient        *fakes.HCSClient
		mounter          *fakes.Mounter
		processClient    *fakes.ProcessClient
		containerManager *container.Manager
		fakeContainer    *hcsfakes.Container
		rootDir          string
	)

	BeforeEach(func() {
		var err error

		rootDir, err = ioutil.TempDir("", "delete.root")
		Expect(err).ToNot(HaveOccurred())

		stateDir := filepath.Join(rootDir, containerId)
		Expect(os.MkdirAll(stateDir, 0755)).To(Succeed())

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

		fakeContainer = &hcsfakes.Container{}
		fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{
			{ProcessId: 666, ImageName: "wininit.exe"},
		}, nil)
		hcsClient.OpenContainerReturns(fakeContainer, nil)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(rootDir)).To(Succeed())
	})

	It("calls the client with the correct container id", func() {
		_, err := containerManager.State()
		Expect(err).NotTo(HaveOccurred())
		Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
		Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))
	})

	Context("when the specified container exists", func() {
		var (
			expectedState               *specs.State
			expectedContainerProperties hcsshim.ContainerProperties
		)

		BeforeEach(func() {
			expectedState = &specs.State{
				Version: specs.Version,
				ID:      containerId,
				Bundle:  bundlePath,
				Pid:     666,
			}

			expectedContainerProperties = hcsshim.ContainerProperties{
				ID:   containerId,
				Name: bundlePath,
			}
			hcsClient.GetContainerPropertiesReturns(expectedContainerProperties, nil)
		})

		Context("when the container has just been created", func() {
			BeforeEach(func() {
				expectedState.Status = "created"
			})

			It("returns the correct state", func() {
				actualState, err := containerManager.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(actualState).To(Equal(expectedState))
				Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
				Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))
				Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))
				Expect(fakeContainer.ProcessListCallCount()).To(Equal(1))
			})
		})

		Context("when the container has been stopped", func() {
			BeforeEach(func() {
				expectedState.Status = "stopped"
				expectedContainerProperties.Stopped = true
				hcsClient.GetContainerPropertiesReturns(expectedContainerProperties, nil)
			})

			It("returns the correct state", func() {
				actualState, err := containerManager.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(actualState).To(Equal(expectedState))
				Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
				Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))
				Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))
				Expect(fakeContainer.ProcessListCallCount()).To(Equal(1))
			})
		})

		Context("when the user process failed to start", func() {
			BeforeEach(func() {
				expectedState.Status = "exited"
				state := container.State{Bundle: bundlePath, UserProgramExecFailed: true}
				contents, err := json.Marshal(state)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.WriteFile(filepath.Join(rootDir, containerId, "state.json"), contents, 0644)).To(Succeed())
			})

			It("returns the correct state", func() {
				actualState, err := containerManager.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(actualState).To(Equal(expectedState))
				Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
				Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))
			})
		})

		Context("when the user process runs and exits", func() {
			BeforeEach(func() {
				expectedState.Status = "running"
				fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{
					hcsshim.ProcessListItem{ProcessId: 100},
					hcsshim.ProcessListItem{ProcessId: 666, ImageName: "wininit.exe"},
				}, nil)
				processClient.StartTimeReturns(syscall.Filetime{LowDateTime: 100, HighDateTime: 20}, nil)
				state := container.State{Bundle: bundlePath, UserProgramPID: 100, UserProgramStartTime: syscall.Filetime{LowDateTime: 100, HighDateTime: 20}}
				contents, err := json.Marshal(state)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.WriteFile(filepath.Join(rootDir, containerId, "state.json"), contents, 0644)).To(Succeed())
			})

			It("returns the correct state", func() {
				actualState, err := containerManager.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(actualState).To(Equal(expectedState))
				Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
				Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))
				Expect(hcsClient.OpenContainerCallCount()).To(Equal(2))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))
				Expect(hcsClient.OpenContainerArgsForCall(1)).To(Equal(containerId))
				Expect(fakeContainer.CloseCallCount()).To(Equal(2))
				Expect(fakeContainer.ProcessListCallCount()).To(Equal(2))
			})

			Context("when the process client fails to get the process start", func() {
				BeforeEach(func() {
					fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{
						hcsshim.ProcessListItem{ProcessId: 100},
						hcsshim.ProcessListItem{ProcessId: 666, ImageName: "wininit.exe"},
					}, nil)
					processClient.StartTimeReturns(syscall.Filetime{}, errors.New("failed to get process start time"))
					state := container.State{Bundle: bundlePath, UserProgramPID: 100, UserProgramStartTime: syscall.Filetime{LowDateTime: 100, HighDateTime: 20}}
					contents, err := json.Marshal(state)
					Expect(err).NotTo(HaveOccurred())
					Expect(ioutil.WriteFile(filepath.Join(rootDir, containerId, "state.json"), contents, 0644)).To(Succeed())
				})

				It("returns the error", func() {
					_, err := containerManager.State()
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the user process runs and exits", func() {
			BeforeEach(func() {
				expectedState.Status = "exited"
				state := container.State{Bundle: bundlePath, UserProgramPID: 100, UserProgramStartTime: syscall.Filetime{LowDateTime: 100, HighDateTime: 20}}
				contents, err := json.Marshal(state)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.WriteFile(filepath.Join(rootDir, containerId, "state.json"), contents, 0644)).To(Succeed())
			})

			It("returns the correct state", func() {
				actualState, err := containerManager.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(actualState).To(Equal(expectedState))
				Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
				Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))
				Expect(hcsClient.OpenContainerCallCount()).To(Equal(2))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))
				Expect(hcsClient.OpenContainerArgsForCall(1)).To(Equal(containerId))
				Expect(fakeContainer.CloseCallCount()).To(Equal(2))
				Expect(fakeContainer.ProcessListCallCount()).To(Equal(2))
			})
		})

		Context("when there are no wininit.exe processes in the container", func() {
			BeforeEach(func() {
				fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{}, nil)
			})

			It("returns 0 as the pid", func() {
				actualState, err := containerManager.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(actualState.Pid).To(Equal(0))
			})
		})

		Context("when there are multiple wininit.exe processes in the container", func() {
			BeforeEach(func() {
				now := time.Now()
				fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{
					{ProcessId: 668, ImageName: "wininit.exe", CreateTimestamp: now.AddDate(0, -1, 0)},
					{ProcessId: 667, ImageName: "wininit.exe", CreateTimestamp: now.AddDate(0, -2, 0)},
					{ProcessId: 666, ImageName: "wininit.exe", CreateTimestamp: now},
				}, nil)
			})

			It("returns the pid of the oldest one as the container pid", func() {
				actualState, err := containerManager.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(actualState.Pid).To(Equal(667))
			})
		})
	})

	Context("when the specified container does not exist", func() {
		var missingContainerError = errors.New("container does not exist")

		BeforeEach(func() {
			hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{}, missingContainerError)
		})

		It("errors", func() {
			_, err := containerManager.State()
			Expect(err).To(Equal(missingContainerError))
		})
	})
})
