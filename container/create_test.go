package container_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/containerfakes"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/hcsclient/hcsclientfakes"
	"code.cloudfoundry.org/winc/network/networkfakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const imageDepot = `C:\var\vcap\data\winc-image\depot`

var _ = Describe("Create", func() {
	var (
		containerId      string
		bundlePath       string
		layerFolders     []string
		hcsClient        *hcsclientfakes.FakeClient
		mounter          *containerfakes.FakeMounter
		networkManager   *networkfakes.FakeNetworkManager
		containerManager container.ContainerManager
		spec             *specs.Spec
	)

	BeforeEach(func() {
		var err error
		bundlePath, err = ioutil.TempDir("", "bundlePath")
		Expect(err).ToNot(HaveOccurred())

		containerId = filepath.Base(bundlePath)

		hcsClient = &hcsclientfakes.FakeClient{}
		mounter = &containerfakes.FakeMounter{}
		networkManager = &networkfakes.FakeNetworkManager{}
		containerManager = container.NewManager(hcsClient, mounter, networkManager, bundlePath)

		networkManager.AttachEndpointToConfigStub = func(config hcsshim.ContainerConfig, containerId string) (hcsshim.ContainerConfig, error) {
			config.EndpointList = []string{"endpoint-for-" + containerId}
			return config, nil
		}

		layerFolders = []string{
			"some-layer",
			"some-other-layer",
			"some-rootfs",
		}

		spec = &specs.Spec{
			Root: &specs.Root{},
			Windows: &specs.Windows{
				LayerFolders: layerFolders,
			},
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	// TODO: fill in other happy path checks and error corner cases

	Context("when the specified container does not already exist", func() {
		var (
			expectedHcsshimLayers []hcsshim.Layer
			fakeContainer         hcsclientfakes.FakeContainer
			containerVolume       = "containervolume"
		)

		BeforeEach(func() {
			fakeContainer = hcsclientfakes.FakeContainer{}
			hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{}, &hcsclient.NotFoundError{})
			hcsClient.GetLayerMountPathReturns(containerVolume, nil)

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

			actualDriverInfo, actualContainerId := hcsClient.GetLayerMountPathArgsForCall(0)
			Expect(actualDriverInfo.HomeDir).To(Equal(imageDepot))
			Expect(actualContainerId).To(Equal(containerId))

			Expect(hcsClient.CreateContainerCallCount()).To(Equal(1))
			actualContainerId, containerConfig := hcsClient.CreateContainerArgsForCall(0)
			Expect(actualContainerId).To(Equal(containerId))
			Expect(containerConfig).To(Equal(&hcsshim.ContainerConfig{
				SystemType:        "Container",
				Name:              bundlePath,
				VolumePath:        containerVolume,
				Owner:             "winc",
				LayerFolderPath:   filepath.Join(imageDepot, containerId),
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
			BeforeEach(func() {
				hcsClient.GetLayerMountPathReturns("", nil)
			})

			It("returns an error", func() {
				err := containerManager.Create(spec)
				Expect(err).To(Equal(&hcsclient.MissingVolumePathError{Id: containerId}))
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
				endpointIds, actualContainerId := networkManager.DeleteEndpointsByIdArgsForCall(0)
				Expect(endpointIds).To(Equal([]string{"endpoint-for-" + containerId}))
				Expect(actualContainerId).To(Equal(containerId))
			})
		})

		Context("when mounting the sandbox.vhdx fails", func() {
			BeforeEach(func() {
				mounter.MountReturns(errors.New("couldn't mount"))
			})

			It("deletes the container and network endpoints", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				Expect(networkManager.DeleteContainerEndpointsCallCount()).To(Equal(1))
				container, actualContainerId := networkManager.DeleteContainerEndpointsArgsForCall(0)
				Expect(container).To(Equal(&fakeContainer))
				Expect(actualContainerId).To(Equal(containerId))
			})
		})

		Context("when container Start fails", func() {
			BeforeEach(func() {
				fakeContainer.StartReturns(errors.New("couldn't start"))
			})

			It("deletes the container and network endpoints", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				Expect(networkManager.DeleteContainerEndpointsCallCount()).To(Equal(1))
				container, actualContainerId := networkManager.DeleteContainerEndpointsArgsForCall(0)
				Expect(container).To(Equal(&fakeContainer))
				Expect(actualContainerId).To(Equal(containerId))
			})
		})

		Context("when getting container pid fails", func() {
			BeforeEach(func() {
				hcsClient.OpenContainerReturns(nil, errors.New("couldn't get pid"))
			})

			It("deletes the container and network endpoints", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				Expect(networkManager.DeleteContainerEndpointsCallCount()).To(Equal(1))
				container, actualContainerId := networkManager.DeleteContainerEndpointsArgsForCall(0)
				Expect(container).To(Equal(&fakeContainer))
				Expect(actualContainerId).To(Equal(containerId))
			})
		})
	})
})
