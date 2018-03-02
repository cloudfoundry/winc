package state_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

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
		containerId = "some-container-id"
		bundlePath  = "some-bundle-path"
	)

	var (
		hcsClient     *fakes.HCSClient
		rootDir       string
		sm            *state.Manager
		fakeContainer *hcsfakes.Container
	)

	BeforeEach(func() {
		var err error

		rootDir, err = ioutil.TempDir("", "winc.container.state.test")
		Expect(err).NotTo(HaveOccurred())

		hcsClient = &fakes.HCSClient{}
		sm = state.NewManager(hcsClient, containerId, rootDir)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(rootDir)).To(Succeed())
	})

	Context("ContainerPid", func() {
		Context("when there are no wininit.exe processes in the container", func() {
			BeforeEach(func() {
				fakeContainer = &hcsfakes.Container{}
				hcsClient.OpenContainerReturnsOnCall(0, fakeContainer, nil)
				fakeContainer.ProcessListReturnsOnCall(0, []hcsshim.ProcessListItem{}, nil)
			})

			It("returns 0 as the pid", func() {
				pid, err := sm.ContainerPid(containerId)
				Expect(err).NotTo(HaveOccurred())
				Expect(pid).To(Equal(0))
			})
		})

		Context("when there are multiple wininit.exe processes in the container", func() {
			BeforeEach(func() {
				fakeContainer = &hcsfakes.Container{}
				hcsClient.OpenContainerReturnsOnCall(0, fakeContainer, nil)
				now := time.Now()
				fakeContainer.ProcessListReturnsOnCall(0, []hcsshim.ProcessListItem{
					{ProcessId: 668, ImageName: "wininit.exe", CreateTimestamp: now.AddDate(0, -1, 0)},
					{ProcessId: 667, ImageName: "wininit.exe", CreateTimestamp: now.AddDate(0, -2, 0)},
					{ProcessId: 666, ImageName: "wininit.exe", CreateTimestamp: now},
				}, nil)
			})

			It("returns the pid of the oldest one as the container pid", func() {
				pid, err := sm.ContainerPid(containerId)
				Expect(err).NotTo(HaveOccurred())
				Expect(pid).To(Equal(667))
			})
		})

		Context("when getting container pid fails", func() {
			BeforeEach(func() {
				hcsClient.OpenContainerReturns(nil, errors.New("couldn't get pid"))
				hcsClient.GetContainerPropertiesReturnsOnCall(1, hcsshim.ContainerProperties{Stopped: false}, nil)
			})

			It("returns an error", func() {
				_, err := sm.ContainerPid(containerId)
				//TODO: use more specific error
				Expect(err).To(MatchError("couldn't get pid"))
			})
		})

	})

	Context("Get", func() {
		const (
			containerPid = 99
		)

		var (
			expectedState *specs.State
		)

		BeforeEach(func() {

			expectedState = &specs.State{
				Version: specs.Version,
				ID:      containerId,
				Pid:     containerPid,
				Bundle:  bundlePath,
			}
		})

		Context("before the manager has been initialized", func() {

			It("doing anything else returns an error", func() {
				_, err := sm.Get()
				Expect(err).To(MatchError(&state.FileNotFoundError{Id: containerId}))
				err = sm.SetRunning(0)
				Expect(err).To(MatchError(&state.FileNotFoundError{Id: containerId}))
				err = sm.SetExecFailed()
				Expect(err).To(MatchError(&state.FileNotFoundError{Id: containerId}))
			})

		})

		Context("initializing the manager", func() {
			It("writes the bundle path to state.json in <rootDir>/<containerId>/", func() {
				err := sm.Initialize(bundlePath)
				Expect(err).To(Succeed())

				var state state.ContainerState
				contents, err := ioutil.ReadFile(filepath.Join(rootDir, containerId, "state.json"))
				Expect(err).NotTo(HaveOccurred())
				Expect(json.Unmarshal(contents, &state)).To(Succeed())

				Expect(state.Bundle).To(Equal(bundlePath))
			})
		})

		Context("after the manager has been initialized", func() {
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
			})

			Context("when OpenContainer successfully returns", func() {
				BeforeEach(func() {
					hcsClient.OpenContainerReturnsOnCall(0, container, nil)
				})

				It("returns the status as 'created' along with the other expected state fields", func() {
					expectedState.Status = "created"
					state, err := sm.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(state).To(Equal(expectedState))

					Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
					Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))
					Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
					Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

					Expect(container.ProcessListCallCount()).To(Equal(1))
					Expect(container.CloseCallCount()).To(Equal(1))
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

						Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
						Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))
						Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
						Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

						Expect(container.ProcessListCallCount()).To(Equal(1))
						Expect(container.CloseCallCount()).To(Equal(1))
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
			})

			Context("FAILURE", func() {
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

			})
		})
	})
})
