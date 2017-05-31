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
		containerManager     container.ContainerManager
		expectedQuery        hcsshim.ComputeSystemQuery
		spec                 *specs.Spec
	)

	BeforeEach(func() {
		var err error

		expectedBundlePath, err = ioutil.TempDir("", "sandbox")
		expectedContainerId = filepath.Base(expectedBundlePath)

		hcsClient = &hcsclientfakes.FakeClient{}
		sandboxManager = &sandboxfakes.FakeSandboxManager{}
		containerManager = container.NewManager(hcsClient, sandboxManager, expectedContainerId)

		expectedQuery = hcsshim.ComputeSystemQuery{
			IDs:    []string{expectedContainerId},
			Owners: []string{"winc"},
		}
		expectedParentLayers = []byte(`["path1", "path2"]`)

		Expect(err).ToNot(HaveOccurred())
		Expect(ioutil.WriteFile(filepath.Join(expectedBundlePath, "layerchain.json"), expectedParentLayers, 0755)).To(Succeed())

		spec = &specs.Spec{}
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
			hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{}, &hcsclient.NotFoundError{})
			hcsClient.GetLayerMountPathReturns(containerVolume, nil)
			sandboxManager.BundlePathReturns(expectedBundlePath)

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
		})

		It("creates and starts it", func() {
			Expect(containerManager.Create(spec)).To(Succeed())

			Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
			Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(expectedContainerId))

			Expect(sandboxManager.CreateCallCount()).To(Equal(1))
			Expect(sandboxManager.BundlePathCallCount()).To(Equal(1))

			Expect(hcsClient.NameToGuidCallCount()).To(Equal(len(expectedLayerPaths)))
			for i, l := range expectedLayerPaths {
				Expect(hcsClient.NameToGuidArgsForCall(i)).To(Equal(l))
			}

			Expect(hcsClient.GetLayerMountPathCallCount()).To(Equal(1))
			driverInfo, containerId := hcsClient.GetLayerMountPathArgsForCall(0)
			Expect(driverInfo).To(Equal(hcsshim.DriverInfo{
				HomeDir: filepath.Dir(expectedBundlePath),
				Flavour: 1,
			}))
			Expect(containerId).To(Equal(expectedContainerId))

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
			}))

			Expect(fakeContainer.StartCallCount()).To(Equal(1))
		})

		Context("when mounts are specified in the spec", func() {
			var expectedMappedDirs []hcsshim.MappedDir

			BeforeEach(func() {
				spec.Mounts = []specs.Mount{
					{Source: "foo", Destination: "bar"},
					{Source: "baz", Destination: "zoo"},
				}

				expectedMappedDirs = []hcsshim.MappedDir{
					{HostPath: "foo", ContainerPath: "bar", ReadOnly: true},
					{HostPath: "baz", ContainerPath: "zoo", ReadOnly: true},
				}
			})

			It("creates the container with the specified mounts", func() {
				Expect(containerManager.Create(spec)).To(Succeed())

				Expect(hcsClient.CreateContainerCallCount()).To(Equal(1))
				containerId, containerConfig := hcsClient.CreateContainerArgsForCall(0)
				Expect(containerId).To(Equal(expectedContainerId))
				Expect(containerConfig.MappedDirectories).To(ConsistOf(expectedMappedDirs))
			})
		})

		Context("when the base of the bundlePath and container id do not match", func() {
			BeforeEach(func() {
				sandboxManager.BundlePathReturns("C:\\notthesamecontainerid")
			})

			It("errors", func() {
				Expect(containerManager.Create(spec)).To(Equal(&hcsclient.InvalidIdError{Id: expectedContainerId}))
			})
		})

		Context("when CreateContainer fails", func() {
			BeforeEach(func() {
				hcsClient.CreateContainerReturns(nil, errors.New("couldn't create"))
			})

			It("deletes the sandbox", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(sandboxManager.DeleteCallCount()).To(Equal(1))
			})
		})

		Context("when container Start fails", func() {
			BeforeEach(func() {
				fakeContainer.StartReturns(errors.New("couldn't start"))
			})

			It("deletes the container and the sandbox", func() {
				Expect(containerManager.Create(spec)).NotTo(Succeed())
				Expect(fakeContainer.TerminateCallCount()).To(Equal(1))
				Expect(sandboxManager.DeleteCallCount()).To(Equal(1))
			})
		})

		Context("when getting the volume mount path of the container fails", func() {
			Context("when getting the volume returned an error", func() {
				var layerMountPathError = errors.New("could not get volume")

				BeforeEach(func() {
					hcsClient.GetLayerMountPathReturns("", layerMountPathError)
				})

				It("errors", func() {
					Expect(containerManager.Create(spec)).To(Equal(layerMountPathError))
				})
			})

			Context("when the volume returned is empty", func() {
				BeforeEach(func() {
					hcsClient.GetLayerMountPathReturns("", nil)
				})
				It("errors", func() {
					Expect(containerManager.Create(spec)).To(Equal(&hcsclient.MissingVolumePathError{Id: expectedContainerId}))
				})
			})
		})
	})
})
