package container_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/containerfakes"
	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/hcs/hcsfakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const rootPath = "some-winc-root-path"

var _ = Describe("Create", func() {
	var (
		containerId      string
		bundlePath       string
		layerFolders     []string
		hcsClient        *containerfakes.FakeHCSClient
		mounter          *containerfakes.FakeMounter
		networkManager   *containerfakes.FakeNetworkManager
		containerManager *container.Manager
		spec             *specs.Spec
		containerVolume  = "containervolume"
		hostName         = "some-hostname"
	)

	BeforeEach(func() {
		var err error
		bundlePath, err = ioutil.TempDir("", "bundlePath")
		Expect(err).ToNot(HaveOccurred())

		containerId = filepath.Base(bundlePath)

		hcsClient = &containerfakes.FakeHCSClient{}
		mounter = &containerfakes.FakeMounter{}
		networkManager = &containerfakes.FakeNetworkManager{}
		containerManager = container.NewManager(hcsClient, mounter, networkManager, rootPath, bundlePath)

		networkManager.AttachEndpointToConfigStub = func(config hcsshim.ContainerConfig) (hcsshim.ContainerConfig, error) {
			config.EndpointList = []string{"endpoint-for-" + containerId}
			return config, nil
		}

		layerFolders = []string{
			"some-layer",
			"some-other-layer",
			"some-rootfs",
		}

		spec = &specs.Spec{
			Root: &specs.Root{
				Path: containerVolume,
			},
			Windows: &specs.Windows{
				LayerFolders: layerFolders,
			},
			Hostname: hostName,
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Context("when the specified container does not already exist", func() {
		var (
			expectedHcsshimLayers []hcsshim.Layer
			fakeContainer         hcsfakes.FakeContainer
		)

		BeforeEach(func() {
			fakeContainer = hcsfakes.FakeContainer{}
			hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{}, &hcs.NotFoundError{})

			expectedHcsshimLayers = []hcsshim.Layer{}
			for i, l := range layerFolders {
				guid := hcsshim.NewGUID(fmt.Sprintf("layer-%d", i))
				hcsClient.NameToGuidReturnsOnCall(i, *guid, nil)
				expectedHcsshimLayers = append(expectedHcsshimLayers, hcsshim.Layer{
					ID:   guid.ToString(),
					Path: l,
				})
			}

			hcsClient.CreateContainerReturns(&fakeContainer, nil)
			hcsClient.OpenContainerReturns(&fakeContainer, nil)
		})

		It("creates and starts it", func() {
			pid := 42
			fakeContainer.ProcessListReturns([]hcsshim.ProcessListItem{
				{ProcessId: uint32(pid), ImageName: "wininit.exe"},
			}, nil)

			Expect(containerManager.Create(spec)).To(Succeed())

			Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
			Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(containerId))

			Expect(hcsClient.NameToGuidCallCount()).To(Equal(len(layerFolders)))
			for i, l := range layerFolders {
				Expect(hcsClient.NameToGuidArgsForCall(i)).To(Equal(filepath.Base(l)))
			}

			Expect(hcsClient.CreateContainerCallCount()).To(Equal(1))
			actualContainerId, containerConfig := hcsClient.CreateContainerArgsForCall(0)
			Expect(actualContainerId).To(Equal(containerId))
			Expect(containerConfig).To(Equal(&hcsshim.ContainerConfig{
				SystemType:        "Container",
				Name:              bundlePath,
				HostName:          hostName,
				VolumePath:        containerVolume,
				Owner:             "winc",
				LayerFolderPath:   filepath.Join(rootPath, containerId),
				Layers:            expectedHcsshimLayers,
				MappedDirectories: []hcsshim.MappedDir{},
				EndpointList:      []string{"endpoint-for-" + containerId},
			}))

			Expect(fakeContainer.StartCallCount()).To(Equal(1))

			Expect(mounter.MountCallCount()).To(Equal(1))
			actualPid, actualVolumePath := mounter.MountArgsForCall(0)
			Expect(actualPid).To(Equal(pid))
			Expect(actualVolumePath).To(Equal(containerVolume))
		})

		Context("when the volume path is empty", func() {
			JustBeforeEach(func() {
				spec.Root.Path = ""
			})

			It("returns an error", func() {
				err := containerManager.Create(spec)
				Expect(err).To(Equal(&container.MissingVolumePathError{Id: containerId}))
			})
		})

		Context("when mounts are specified in the spec", func() {
			var (
				expectedMappedDirs []hcsshim.MappedDir
				mount              string
			)

			BeforeEach(func() {
				var err error
				mount, err = ioutil.TempDir("", "mountdir")
				Expect(err).ToNot(HaveOccurred())

				spec.Mounts = []specs.Mount{
					{Source: mount, Destination: "/bar"},
				}

				expectedMappedDirs = []hcsshim.MappedDir{
					{HostPath: mount, ContainerPath: "C:\\bar", ReadOnly: true},
				}
			})

			AfterEach(func() {
				Expect(os.RemoveAll(mount)).To(Succeed())
			})

			It("creates the container with the specified mounts", func() {
				Expect(containerManager.Create(spec)).To(Succeed())

				Expect(hcsClient.CreateContainerCallCount()).To(Equal(1))
				actualContainerId, containerConfig := hcsClient.CreateContainerArgsForCall(0)
				Expect(actualContainerId).To(Equal(containerId))
				Expect(containerConfig.MappedDirectories).To(ConsistOf(expectedMappedDirs))
			})

			Context("when the mount does not exist", func() {
				BeforeEach(func() {
					Expect(os.RemoveAll(mount)).To(Succeed())
				})

				It("errors", func() {
					err := containerManager.Create(spec)
					Expect(os.IsNotExist(err)).To(BeTrue())
				})
			})

			Context("when a file is specified as a mount", func() {
				var mountFile string

				BeforeEach(func() {
					m, err := ioutil.TempFile("", "mountfile")
					Expect(err).ToNot(HaveOccurred())
					Expect(m.Close()).To(Succeed())
					mountFile = m.Name()

					spec.Mounts = append(spec.Mounts, specs.Mount{
						Source:      mountFile,
						Destination: "foo",
					})
				})

				AfterEach(func() {
					Expect(os.RemoveAll(mountFile)).To(Succeed())
				})

				It("ignores it", func() {
					Expect(containerManager.Create(spec)).To(Succeed())

					Expect(hcsClient.CreateContainerCallCount()).To(Equal(1))
					actualContainerId, containerConfig := hcsClient.CreateContainerArgsForCall(0)
					Expect(actualContainerId).To(Equal(containerId))
					Expect(containerConfig.MappedDirectories).To(ConsistOf(expectedMappedDirs))
				})
			})
		})

		Context("when memory limits are specified in the spec", func() {
			var expectedMemoryMaxinMB uint64

			BeforeEach(func() {
				expectedMemoryMaxinMB = uint64(64)
				expectedMemoryMaxinBytes := expectedMemoryMaxinMB * 1024 * 1024
				spec.Windows.Resources = &specs.WindowsResources{
					Memory: &specs.WindowsMemoryResources{
						Limit: &expectedMemoryMaxinBytes,
					},
				}
			})

			It("creates the container with the specified memory limits", func() {
				Expect(containerManager.Create(spec)).To(Succeed())

				Expect(hcsClient.CreateContainerCallCount()).To(Equal(1))
				_, containerConfig := hcsClient.CreateContainerArgsForCall(0)
				Expect(containerConfig.MemoryMaximumInMB).To(Equal(int64(expectedMemoryMaxinMB)))
			})
		})

		Context("when cpu limits are specified in the spec", func() {
			var expectedCPUShares uint16

			BeforeEach(func() {
				expectedCPUShares = 8080
				spec.Windows.Resources = &specs.WindowsResources{
					CPU: &specs.WindowsCPUResources{
						Shares: &expectedCPUShares,
					},
				}
			})

			It("creates the container with the specified cpu limits", func() {
				Expect(containerManager.Create(spec)).To(Succeed())

				Expect(hcsClient.CreateContainerCallCount()).To(Equal(1))
				_, containerConfig := hcsClient.CreateContainerArgsForCall(0)
				Expect(containerConfig.ProcessorWeight).To(Equal(uint64(expectedCPUShares)))
			})
		})

		Context("when attaching endpoint fails", func() {
			var attachError error

			BeforeEach(func() {
				attachError = errors.New("couldn't attach")
				networkManager.AttachEndpointToConfigReturns(hcsshim.ContainerConfig{}, attachError)
			})

			It("errors", func() {
				Expect(containerManager.Create(spec)).To(Equal(attachError))
			})
		})

		Context("when CreateContainer fails", func() {
			BeforeEach(func() {
				hcsClient.CreateContainerReturns(nil, errors.New("couldn't create"))
			})

			It("deletes the network endpoints", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(networkManager.DeleteEndpointsByIdCallCount()).To(Equal(1))
				endpointIds := networkManager.DeleteEndpointsByIdArgsForCall(0)
				Expect(endpointIds).To(Equal([]string{"endpoint-for-" + containerId}))
			})
		})

		Context("when mounting the sandbox.vhdx fails", func() {
			BeforeEach(func() {
				mounter.MountReturns(errors.New("couldn't mount"))
				hcsClient.GetContainerPropertiesReturnsOnCall(1, hcsshim.ContainerProperties{Stopped: false}, nil)
			})

			It("deletes the container and network endpoints", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				Expect(networkManager.DeleteContainerEndpointsCallCount()).To(Equal(1))
				container := networkManager.DeleteContainerEndpointsArgsForCall(0)
				Expect(container).To(Equal(&fakeContainer))
			})
		})

		Context("when container Start fails", func() {
			BeforeEach(func() {
				fakeContainer.StartReturns(errors.New("couldn't start"))
				hcsClient.GetContainerPropertiesReturnsOnCall(1, hcsshim.ContainerProperties{Stopped: true}, nil)
			})

			It("deletes network endpoints but skips the container shutdown", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(fakeContainer.CloseCallCount()).To(Equal(1))
				Expect(fakeContainer.ShutdownCallCount()).To(Equal(0))
				Expect(fakeContainer.TerminateCallCount()).To(Equal(0))
				Expect(networkManager.DeleteContainerEndpointsCallCount()).To(Equal(1))
				container := networkManager.DeleteContainerEndpointsArgsForCall(0)
				Expect(container).To(Equal(&fakeContainer))
			})
		})

		Context("when getting container pid fails", func() {
			BeforeEach(func() {
				hcsClient.OpenContainerReturns(nil, errors.New("couldn't get pid"))
				hcsClient.GetContainerPropertiesReturnsOnCall(1, hcsshim.ContainerProperties{Stopped: false}, nil)
			})

			It("deletes the container and network endpoints", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				Expect(networkManager.DeleteContainerEndpointsCallCount()).To(Equal(1))
				container := networkManager.DeleteContainerEndpointsArgsForCall(0)
				Expect(container).To(Equal(&fakeContainer))
			})
		})
	})
})
