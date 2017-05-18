package container_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/containerfakes"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/hcsclient/hcsclientfakes"
	"code.cloudfoundry.org/winc/sandbox/sandboxfakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create", func() {
	const (
		rootfs          = "C:\\rootfs"
		containerVolume = "containervolume"
	)

	var (
		expectedContainerId string
		expectedBundlePath  string
		hcsClient           *hcsclientfakes.FakeClient
		sandboxManager      *sandboxfakes.FakeSandboxManager
		fakeContainer       *containerfakes.FakeHCSContainer
		containerManager    container.ContainerManager
		expectedQuery       hcsshim.ComputeSystemQuery
	)

	BeforeEach(func() {
		var err error

		expectedBundlePath, err = ioutil.TempDir("", "sandbox")
		expectedContainerId = filepath.Base(expectedBundlePath)

		hcsClient = &hcsclientfakes.FakeClient{}
		sandboxManager = &sandboxfakes.FakeSandboxManager{}
		fakeContainer = &containerfakes.FakeHCSContainer{}
		containerManager = container.NewManager(hcsClient, sandboxManager, expectedContainerId)

		expectedQuery = hcsshim.ComputeSystemQuery{
			IDs:    []string{expectedContainerId},
			Owners: []string{"winc"},
		}

		Expect(err).ToNot(HaveOccurred())
		Expect(ioutil.WriteFile(filepath.Join(expectedBundlePath, "layerchain.json"), []byte(`["hello"]`), 0755)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(expectedBundlePath)).To(Succeed())
	})

	// TODO: fill in other happy path checks and error corner cases

	Context("when the specified container does not already exist", func() {
		BeforeEach(func() {
			hcsClient.GetContainerPropertiesReturns(hcsshim.ContainerProperties{}, &hcsclient.NotFoundError{})
			hcsClient.GetLayerMountPathReturns(containerVolume, nil)
			sandboxManager.BundlePathReturns(expectedBundlePath)
		})

		It("creates it", func() {
			Expect(containerManager.Create(rootfs)).To(Succeed())

			Expect(hcsClient.GetContainerPropertiesCallCount()).To(Equal(1))
			Expect(hcsClient.GetContainerPropertiesArgsForCall(0)).To(Equal(expectedContainerId))

			Expect(sandboxManager.CreateCallCount()).To(Equal(1))
			Expect(sandboxManager.BundlePathCallCount()).To(Equal(1))

			// Expect(hcsClient.IsPendingCallCount()).To(Equal(1))
			// Expect(hcsClient.IsPendingArgsForCall(0)).To(BeNil())

			// Expect(sandboxManager.DeleteCallCount()).To(Equal(1))
		})

		Context("when the base of the bundlePath and container id do not match", func() {
			BeforeEach(func() {
				sandboxManager.BundlePathReturns("C:\\notthesamecontainerid")
			})

			It("errors", func() {
				Expect(containerManager.Create(rootfs)).To(Equal(&hcsclient.InvalidIdError{Id: expectedContainerId}))
			})
		})

		Context("when getting the volume mount path of the container fails", func() {
			XContext("when getting the volume returned an error", func() {
				It("errors", func() {
				})
			})

			Context("when the volume returned is empty", func() {
				BeforeEach(func() {
					hcsClient.GetLayerMountPathReturns("", nil)
				})
				It("errors", func() {
					Expect(containerManager.Create(rootfs)).To(Equal(&hcsclient.MissingVolumePathError{Id: expectedContainerId}))
				})
			})
		})
	})
})
