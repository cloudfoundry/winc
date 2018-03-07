package container_test

import (
	"fmt"
	"io/ioutil"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/fakes"
	"code.cloudfoundry.org/winc/container/state"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("State", func() {
	const (
		containerId = "some-id"
	)

	var (
		hcsClient        *fakes.HCSClient
		mounter          *fakes.Mounter
		stateManager     *fakes.StateManager
		processManager   *fakes.ProcessManager
		containerManager *container.Manager
	)

	BeforeEach(func() {
		hcsClient = &fakes.HCSClient{}
		mounter = &fakes.Mounter{}
		stateManager = &fakes.StateManager{}
		processManager = &fakes.ProcessManager{}
		logger := (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "state")

		containerManager = container.NewManager(logger, hcsClient, mounter, stateManager, containerId, "", processManager)
	})

	Context("when the state manager returns the state successfully", func() {
		var expectedState specs.State
		var status string
		var bundlePath string
		BeforeEach(func() {
			expectedState = specs.State{
				Version: specs.Version,
				ID:      containerId,
				Status:  "some-status",
				Bundle:  "some-bundle-path",
				Pid:     99,
			}

			status = "some-status"
			bundlePath = "some-bundle-path"
			stateManager.GetReturnsOnCall(0, status, bundlePath, nil)
			processManager.ContainerPidReturnsOnCall(0, 99, nil)
		})

		It("calls the state manager and returns the state", func() {
			state, err := containerManager.State()
			Expect(stateManager.GetCallCount()).To(Equal(1))
			Expect(processManager.ContainerPidCallCount()).To(Equal(1))
			Expect(processManager.ContainerPidArgsForCall(0)).To(Equal(containerId))
			Expect(err).NotTo(HaveOccurred())
			Expect(state).To(Equal(&expectedState))
		})
	})

	Context("when the state manager fails to return the state", func() {
		Context("when the state file cannot be found", func() {
			BeforeEach(func() {
				stateManager.GetReturnsOnCall(0, "", "", fmt.Errorf("error getting state"))
			})

			It("calls the state manager and returns the error", func() {
				_, err := containerManager.State()
				Expect(stateManager.GetCallCount()).To(Equal(1))
				Expect(err).To(MatchError("error getting state"))
			})
		})

		Context("when the container cannot be found", func() {
			BeforeEach(func() {
				processManager.ContainerPidReturnsOnCall(0, -1, &state.ContainerNotFoundError{Id: "some-id"})
			})

			It("calls the state manager and returns the error", func() {
				_, err := containerManager.State()
				Expect(stateManager.GetCallCount()).To(Equal(1))
				Expect(err).To(MatchError("container not found: some-id"))
			})
		})
	})
})
