package container_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
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
		logger := (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "state")

		containerManager = container.NewManager(logger, hcsClient, mounter, containerId, rootDir)

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
			actualState                 *specs.State
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
		})

		JustBeforeEach(func() {
			var err error
			hcsClient.GetContainerPropertiesReturns(expectedContainerProperties, nil)
			actualState, err = containerManager.State()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the container has just been created", func() {
			BeforeEach(func() {
				expectedState.Status = "created"
			})

			It("returns the correct state", func() {
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
			})

			It("returns the correct state", func() {
				Expect(actualState).To(Equal(expectedState))
				Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
				Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))
				Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))
				Expect(fakeContainer.ProcessListCallCount()).To(Equal(1))
			})
		})

		Context("when there are no wininit.exe processes in the container", func() {
			BeforeEach(func() {
				fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{}, nil)
			})

			It("returns 0 as the pid", func() {
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
