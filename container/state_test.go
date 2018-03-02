package container_test

import (
	"errors"
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
		containerManager *container.Manager
	)

	BeforeEach(func() {
		hcsClient = &fakes.HCSClient{}
		mounter = &fakes.Mounter{}
		stateManager = &fakes.StateManager{}
		logger := (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "state")

		containerManager = container.NewManager(logger, hcsClient, mounter, stateManager, containerId, "")
	})

	Context("when the state manager returns the state successfully", func() {
		var expectedState specs.State
		BeforeEach(func() {
			expectedState = specs.State{
				Version: specs.Version,
				ID:      "some-container-id",
				Status:  "some-status",
				Bundle:  "some-bundle-path",
				Pid:     99,
			}
			stateManager.GetReturnsOnCall(0, &expectedState, nil)
		})

		It("calls the state manager and returns the state", func() {
			state, err := containerManager.State()
			Expect(stateManager.GetCallCount()).To(Equal(1))
			Expect(err).NotTo(HaveOccurred())
			Expect(state).To(Equal(&expectedState))
		})
	})

	Context("when the state manager fails to return the state", func() {
		Context("when the state file cannot be found", func() {
			BeforeEach(func() {
				stateManager.GetReturnsOnCall(0, nil, &state.FileNotFoundError{Id: "some-id"})
			})

			It("calls the state manager and returns the error", func() {
				_, err := containerManager.State()
				Expect(stateManager.GetCallCount()).To(Equal(1))
				Expect(err).To(MatchError("container not found: some-id"))
			})
		})

		Context("when the state manager cannot find the container", func() {
			panic(errors.New("IMPLEMENT ME"))
		})
	})
})
