package container_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/hcsclient/hcsclientfakes"
	"code.cloudfoundry.org/winc/network/networkfakes"
	"code.cloudfoundry.org/winc/sandbox/sandboxfakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Create", func() {
	const (
		rootfs          = "C:\\rootfs"
		containerVolume = "containervolume"
	)

	var (
		expectedContainerId  string
		expectedBundlePath   string
		expectedParentLayers []byte
		hcsClient            *hcsclientfakes.FakeClient
		sandboxManager       *sandboxfakes.FakeSandboxManager
		networkManager       *networkfakes.FakeNetworkManager
		containerManager     container.ContainerManager
		spec                 *specs.Spec
	)

	BeforeEach(func() {
		var err error
		expectedBundlePath, err = ioutil.TempDir("", "sandbox")
		Expect(err).ToNot(HaveOccurred())

		expectedContainerId = filepath.Base(expectedBundlePath)

		hcsClient = &hcsclientfakes.FakeClient{}
		sandboxManager = &sandboxfakes.FakeSandboxManager{}
		networkManager = &networkfakes.FakeNetworkManager{}
		containerManager = container.NewManager(hcsClient, sandboxManager, networkManager, expectedContainerId)

		expectedParentLayers = []byte(`["path1", "path2"]`)
		networkManager.AttachEndpointToConfigStub = func(config hcsshim.ContainerConfig, containerID string) (hcsshim.ContainerConfig, error) {
			config.EndpointList = []string{"endpoint-for-" + containerID}
			return config, nil
		}

		Expect(ioutil.WriteFile(filepath.Join(expectedBundlePath, "layerchain.json"), expectedParentLayers, 0755)).To(Succeed())

		spec = &specs.Spec{Root: &specs.Root{}}
		spec.Root.Path = rootfs
	})

	AfterEach(func() {
		Expect(os.RemoveAll(expectedBundlePath)).To(Succeed())
	})

	// TODO: fill in other happy path checks and error corner cases

	Context("when the specified container does not already exist", func() {
		var (
			expectedLayerPaths    []string
			expectedHcsshimLayers []hcsshim.Layer
			fakeContainer         hcsclientfakes.FakeContainer
		)

		BeforeEach(func() {
			fakeContainer = hcsclientfakes.FakeContainer{}
			hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{}, &hcsclient.NotFoundError{})
			sandboxManager.BundlePathReturns(expectedBundlePath)
			sandboxManager.CreateReturns(containerVolume, nil)

			err := json.Unmarshal(expectedParentLayers, &expectedLayerPaths)
			Expect(err).ToNot(HaveOccurred())

			layerGuid := hcsshim.NewGUID("layerguid")
			hcsClient.NameToGuidReturns(*layerGuid, nil)
			expectedHcsshimLayers = []hcsshim.Layer{}
			for _, l := range expectedLayerPaths {
				expectedHcsshimLayers = append(expectedHcsshimLayers, hcsshim.Layer{
					ID:   layerGuid.ToString(),
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
			Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(expectedContainerId))

			Expect(sandboxManager.CreateCallCount()).To(Equal(1))
			Expect(sandboxManager.BundlePathCallCount()).To(Equal(1))

			Expect(hcsClient.NameToGuidCallCount()).To(Equal(len(expectedLayerPaths)))
			for i, l := range expectedLayerPaths {
				Expect(hcsClient.NameToGuidArgsForCall(i)).To(Equal(l))
			}

			Expect(hcsClient.CreateContainerCallCount()).To(Equal(1))
			containerId, containerConfig := hcsClient.CreateContainerArgsForCall(0)
			Expect(containerId).To(Equal(expectedContainerId))
			Expect(containerConfig).To(Equal(&hcsshim.ContainerConfig{
				SystemType:        "Container",
				Name:              expectedBundlePath,
				VolumePath:        containerVolume,
				Owner:             "winc",
				LayerFolderPath:   expectedBundlePath,
				Layers:            expectedHcsshimLayers,
				MappedDirectories: []hcsshim.MappedDir{},
				EndpointList:      []string{"endpoint-for-" + expectedContainerId},
			}))

			Expect(fakeContainer.StartCallCount()).To(Equal(1))

			Expect(sandboxManager.MountCallCount()).To(Equal(1))
			Expect(sandboxManager.MountArgsForCall(0)).To(Equal(pid))
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
				containerId, containerConfig := hcsClient.CreateContainerArgsForCall(0)
				Expect(containerId).To(Equal(expectedContainerId))
				Expect(containerConfig.MappedDirectories).To(ConsistOf(expectedMappedDirs))
			})

			Context("when the mount does not exist", func() {
				BeforeEach(func() {
					Expect(os.RemoveAll(mount)).To(Succeed())
				})

				It("errors and removes the sandbox", func() {
					err := containerManager.Create(spec)
					Expect(os.IsNotExist(err)).To(BeTrue())
					Expect(sandboxManager.DeleteCallCount()).To(Equal(1))
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
					containerId, containerConfig := hcsClient.CreateContainerArgsForCall(0)
					Expect(containerId).To(Equal(expectedContainerId))
					Expect(containerConfig.MappedDirectories).To(ConsistOf(expectedMappedDirs))
				})
			})
		})

		Context("when memory limits are specified in the spec", func() {
			var expectedMemoryMaxinMB uint64

			BeforeEach(func() {
				expectedMemoryMaxinMB = uint64(64)
				expectedMemoryMaxinBytes := expectedMemoryMaxinMB * 1024 * 1024
				spec.Windows = &specs.Windows{
					Resources: &specs.WindowsResources{
						Memory: &specs.WindowsMemoryResources{
							Limit: &expectedMemoryMaxinBytes,
						},
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
			BeforeEach(func() {
				networkManager.AttachEndpointToConfigReturns(hcsshim.ContainerConfig{}, errors.New("couldn't attach"))
			})

			It("deletes the sandbox", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(sandboxManager.DeleteCallCount()).To(Equal(1))
			})
		})

		Context("when CreateContainer fails", func() {
			BeforeEach(func() {
				hcsClient.CreateContainerReturns(nil, errors.New("couldn't create"))
			})

			It("deletes the sandbox + endpoint", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(sandboxManager.DeleteCallCount()).To(Equal(1))
				Expect(networkManager.DeleteEndpointsByIdCallCount()).To(Equal(1))
				endpointIds, containerId := networkManager.DeleteEndpointsByIdArgsForCall(0)
				Expect(endpointIds).To(Equal([]string{"endpoint-for-" + expectedContainerId}))
				Expect(containerId).To(Equal(expectedContainerId))
			})
		})

		Context("when mounting the sandbox.vhdx fails", func() {
			BeforeEach(func() {
				sandboxManager.MountReturns(errors.New("couldn't mount"))
			})

			It("deletes the container, sandbox, and endpoints", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				Expect(sandboxManager.DeleteCallCount()).To(Equal(1))
				Expect(networkManager.DeleteContainerEndpointsCallCount()).To(Equal(1))
				container, containerID := networkManager.DeleteContainerEndpointsArgsForCall(0)
				Expect(container).To(Equal(&fakeContainer))
				Expect(containerID).To(Equal(expectedContainerId))
			})
		})

		Context("when container Start fails", func() {
			BeforeEach(func() {
				fakeContainer.StartReturns(errors.New("couldn't start"))
			})

			It("deletes the container, sandbox, and endpoints", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				Expect(sandboxManager.DeleteCallCount()).To(Equal(1))
				Expect(networkManager.DeleteContainerEndpointsCallCount()).To(Equal(1))
				container, containerID := networkManager.DeleteContainerEndpointsArgsForCall(0)
				Expect(container).To(Equal(&fakeContainer))
				Expect(containerID).To(Equal(expectedContainerId))
			})
		})

		Context("when getting container pid fails", func() {
			BeforeEach(func() {
				hcsClient.OpenContainerReturns(nil, errors.New("couldn't get pid"))
			})

			It("deletes the container, sandbox, and endpoints", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(fakeContainer.ShutdownCallCount()).To(Equal(1))
				Expect(sandboxManager.DeleteCallCount()).To(Equal(1))
				Expect(networkManager.DeleteContainerEndpointsCallCount()).To(Equal(1))
				container, containerID := networkManager.DeleteContainerEndpointsArgsForCall(0)
				Expect(container).To(Equal(&fakeContainer))
				Expect(containerID).To(Equal(expectedContainerId))
			})
		})
	})
})
