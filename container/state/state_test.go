package state_test

import (
	"errors"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/winc/container/state"
	"code.cloudfoundry.org/winc/container/state/fakes"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("StateManager", func() {
	const (
		containerId  = "some-container-id"
		containerPid = 99
		bundlePath   = "some-bundle-path"
	)

	var (
		hcsClient     *fakes.HCSClient
		sm            *state.Manager
		rootDir       string
		expectedState *specs.State
	)

	BeforeEach(func() {
		var err error

		rootDir, err = ioutil.TempDir("", "winc.container.state.test")
		Expect(err).NotTo(HaveOccurred())

		hcsClient = &fakes.HCSClient{}
		sm = state.NewManager(hcsClient, containerId, rootDir)

		expectedState = &specs.State{
			Version: specs.Version,
			ID:      containerId,
			Pid:     containerPid,
			Bundle:  bundlePath,
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(rootDir)).To(Succeed())
	})

	Context("before the manager has been initialized", func() {

		It("doing anything else returns an error", func() {
			_, err := sm.Get()
			Expect(err).To(MatchError("manager has not been initialized"))
			err = sm.SetRunning(0)
			Expect(err).To(MatchError("manager has not been initialized"))
			err = sm.SetExecFailed()
			Expect(err).To(MatchError("manager has not been initialized"))
		})
	})

	Context("after the manager has beed initialized", func() {
		var (
			container *hcsfakes.Container
		)

		BeforeEach(func() {
			Expect(sm.Initialize(bundlePath)).To(Succeed())

			container = &hcsfakes.Container{}
			container.ProcessListReturnsOnCall(0, []hcsshim.ProcessListItem{
				hcsshim.ProcessListItem{
					ProcessId: uint32(containerPid),
					ImageName: "wininit.exe",
				},
			}, nil)
			hcsClient.OpenContainerReturnsOnCall(0, container, nil)
		})

		It("returns the status as 'created' along with the other expected state fields", func() {
			expectedState.Status = "created"
			state, err := sm.Get()
			Expect(err).NotTo(HaveOccurred())
			Expect(state).To(Equal(expectedState))

			Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
			Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

			Expect(container.ProcessListCallCount()).To(Equal(1))
			Expect(container.CloseCallCount()).To(Equal(1))
		})

		Context("when the container cannot be opened", func() {
			BeforeEach(func() {
				hcsClient.OpenContainerReturnsOnCall(0, nil, errors.New("cannot open container"))
			})

			It("returns the error", func() {
				_, err := sm.Get()
				Expect(err).To(MatchError("cannot open container"))

				Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))
			})
		})

		Context("after the init process has been successfully started", func() {
			var initProcessPid int
			BeforeEach(func() {
				initProcessPid = 89
				//Expect(sm.SetRunning(initProcessPid)).To(Succeed())

				hcsClient.OpenContainerReturnsOnCall(0, container, nil)
				hcsClient.OpenContainerReturnsOnCall(1, container, nil)
			})

			Context("and the init process is still running", func() {
				BeforeEach(func() {
					processList := []hcsshim.ProcessListItem{
						hcsshim.ProcessListItem{
							ProcessId: uint32(containerPid),
							ImageName: "wininit.exe",
						},
						hcsshim.ProcessListItem{
							ProcessId: uint32(initProcessPid),
							ImageName: "init-process.exe",
						},
					}
					container.ProcessListReturnsOnCall(0, processList, nil)
					container.ProcessListReturnsOnCall(1, processList, nil)
				})

				XIt("returns the status as 'running' along with the other expected state fields", func() {
					expectedState.Status = "running"
					state, err := sm.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(state).To(Equal(expectedState))

					Expect(hcsClient.OpenContainerCallCount()).To(Equal(2))
					Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))
					Expect(hcsClient.OpenContainerArgsForCall(1)).To(Equal(containerId))

					Expect(container.ProcessListCallCount()).To(Equal(2))
					Expect(container.CloseCallCount()).To(Equal(2))
				})
			})

			Context("and the init process has returned", func() {
				BeforeEach(func() {

					processList := []hcsshim.ProcessListItem{
						hcsshim.ProcessListItem{
							ProcessId: uint32(containerPid),
							ImageName: "wininit.exe",
						},
					}
					container.ProcessListReturnsOnCall(0, processList, nil)
				})

				XIt("returns the status as 'exited' along with the other expected state fields", func() {
					expectedState.Status = "exited"
					state, err := sm.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(state).To(Equal(expectedState))

					Expect(hcsClient.OpenContainerCallCount()).To(Equal(2))
					Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))
					Expect(hcsClient.OpenContainerArgsForCall(1)).To(Equal(containerId))

					Expect(container.ProcessListCallCount()).To(Equal(2))
					Expect(container.CloseCallCount()).To(Equal(2))
				})
			})
		})

		Context("after the init process has failed to start", func() {
			BeforeEach(func() {
				Expect(sm.SetExecFailed()).To(Succeed())

				hcsClient.OpenContainerReturnsOnCall(0, container, nil)

				processList := []hcsshim.ProcessListItem{
					hcsshim.ProcessListItem{
						ProcessId: uint32(containerPid),
						ImageName: "wininit.exe",
					},
				}
				container.ProcessListReturnsOnCall(0, processList, nil)
			})

			It("returns the status as 'exited' along with the other expected state fields", func() {
				expectedState.Status = "exited"
				state, err := sm.Get()
				Expect(err).NotTo(HaveOccurred())
				Expect(state).To(Equal(expectedState))

				Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

				Expect(container.ProcessListCallCount()).To(Equal(1))
				Expect(container.CloseCallCount()).To(Equal(1))
			})
		})

		Context("after the container has been stopped", func() {
			BeforeEach(func() {
				hcsClient.GetContainerPropertiesReturnsOnCall(0, hcsshim.ContainerProperties{Stopped: true}, nil)
			})

			It("returns the status as 'stopped' along with the other expected state fields", func() {
				expectedState.Status = "stopped"
				state, err := sm.Get()
				Expect(err).NotTo(HaveOccurred())
				Expect(state).To(Equal(expectedState))

				Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
				Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

				Expect(container.ProcessListCallCount()).To(Equal(1))
				Expect(container.CloseCallCount()).To(Equal(1))
			})
		})
	})
})
