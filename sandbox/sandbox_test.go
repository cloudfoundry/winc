package sandbox_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/hcsclient/hcsclientfakes"
	"code.cloudfoundry.org/winc/sandbox"
	"github.com/Microsoft/hcsshim"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sandbox", func() {
	var (
		bundlePath           string
		rootfs               string
		hcsClient            *hcsclientfakes.FakeClient
		sandboxManager       sandbox.SandboxManager
		expectedDriverInfo   hcsshim.DriverInfo
		expectedLayerId      string
		expectedParentLayer  string
		expectedParentLayers []byte
	)

	BeforeEach(func() {
		var err error
		rootfs, err = ioutil.TempDir("", "rootfs")
		Expect(err).ToNot(HaveOccurred())

		bundlePath, err = ioutil.TempDir("", "sandbox")
		Expect(err).ToNot(HaveOccurred())

		hcsClient = &hcsclientfakes.FakeClient{}
		sandboxManager = sandbox.NewManager(hcsClient, bundlePath)

		expectedDriverInfo = hcsshim.DriverInfo{
			HomeDir: filepath.Dir(bundlePath),
			Flavour: 1,
		}
		expectedLayerId = filepath.Base(bundlePath)
		expectedParentLayer = "path1"
		expectedParentLayers = []byte(`["path1", "path2"]`)
	})

	JustBeforeEach(func() {
		Expect(ioutil.WriteFile(filepath.Join(rootfs, "layerchain.json"), expectedParentLayers, 0755)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
		Expect(os.RemoveAll(rootfs)).To(Succeed())
	})

	Context("Create", func() {
		Context("when provided a rootfs layer", func() {
			It("creates and activates the bundlePath", func() {
				err := sandboxManager.Create(rootfs)
				Expect(err).ToNot(HaveOccurred())

				var expectedLayers []string
				err = json.Unmarshal(expectedParentLayers, &expectedLayers)
				Expect(err).ToNot(HaveOccurred())

				Expect(hcsClient.CreateSandboxLayerCallCount()).To(Equal(1))
				driverInfo, layerId, parentLayer, parentLayers := hcsClient.CreateSandboxLayerArgsForCall(0)
				Expect(driverInfo).To(Equal(expectedDriverInfo))
				Expect(layerId).To(Equal(expectedLayerId))
				Expect(parentLayer).To(Equal(expectedParentLayer))
				Expect(parentLayers).To(Equal(expectedLayers))

				Expect(hcsClient.ActivateLayerCallCount()).To(Equal(1))
				driverInfo, layerId = hcsClient.ActivateLayerArgsForCall(0)
				Expect(driverInfo).To(Equal(expectedDriverInfo))
				Expect(layerId).To(Equal(expectedLayerId))

				Expect(hcsClient.PrepareLayerCallCount()).To(Equal(1))
				driverInfo, layerId, parentLayers = hcsClient.PrepareLayerArgsForCall(0)
				Expect(driverInfo).To(Equal(expectedDriverInfo))
				Expect(layerId).To(Equal(expectedLayerId))
				Expect(parentLayers).To(Equal(expectedLayers))
			})

			Context("when creating the bundlePath fails", func() {
				var createSandboxLayerError = errors.New("create sandbox failed")

				BeforeEach(func() {
					hcsClient.CreateSandboxLayerReturns(createSandboxLayerError)
				})

				It("errors", func() {
					err := sandboxManager.Create(rootfs)
					Expect(err).To(Equal(createSandboxLayerError))
				})
			})

			Context("when activating the bundlePath fails", func() {
				var activateLayerError = errors.New("activate sandbox failed")

				BeforeEach(func() {
					hcsClient.ActivateLayerReturns(activateLayerError)
				})

				It("errors", func() {
					err := sandboxManager.Create(rootfs)
					Expect(err).To(Equal(activateLayerError))
				})
			})

			Context("when preparing the bundlePath fails", func() {
				var prepareLayerError = errors.New("prepare sandbox failed")

				BeforeEach(func() {
					hcsClient.PrepareLayerReturns(prepareLayerError)
				})

				It("errors", func() {
					err := sandboxManager.Create(rootfs)
					Expect(err).To(Equal(prepareLayerError))
				})
			})
		})

		Context("when provided a nonexistent rootfs layer", func() {
			It("errors", func() {
				err := sandboxManager.Create("nonexistentrootfs")
				Expect(err).To(Equal(&sandbox.MissingRootfsError{Msg: "nonexistentrootfs"}))
			})
		})

		Context("when provided a rootfs layer missing a layerchain.json", func() {
			JustBeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(rootfs, "layerchain.json"))).To(Succeed())
			})

			It("errors", func() {
				err := sandboxManager.Create(rootfs)
				Expect(err).To(Equal(&sandbox.MissingRootfsLayerChainError{Msg: rootfs}))
			})
		})

		Context("when the rootfs has a layerchain.json that is invalid JSON", func() {
			BeforeEach(func() {
				expectedParentLayers = []byte("[")
			})

			It("errors", func() {
				err := sandboxManager.Create(rootfs)
				Expect(err).To(Equal(&sandbox.InvalidRootfsLayerChainError{Msg: rootfs}))
			})
		})

		Context("when the bundlePath directory does not exist", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(bundlePath)).To(Succeed())
			})

			It("errors", func() {
				err := sandboxManager.Create(rootfs)
				Expect(err).To(Equal(&sandbox.MissingBundlePathError{Msg: bundlePath}))
			})
		})
	})

	Context("Delete", func() {
		It("deletes and deactivates the bundlePath", func() {
			err := sandboxManager.Delete()
			Expect(err).ToNot(HaveOccurred())

			_, err = os.Stat(bundlePath)
			Expect(os.IsNotExist(err)).To(BeTrue())

			Expect(hcsClient.UnprepareLayerCallCount()).To(Equal(1))
			driverInfo, layerId := hcsClient.UnprepareLayerArgsForCall(0)
			Expect(driverInfo).To(Equal(expectedDriverInfo))
			Expect(layerId).To(Equal(expectedLayerId))

			Expect(hcsClient.DeactivateLayerCallCount()).To(Equal(1))
			driverInfo, layerId = hcsClient.DeactivateLayerArgsForCall(0)
			Expect(driverInfo).To(Equal(expectedDriverInfo))
			Expect(layerId).To(Equal(expectedLayerId))

			Expect(hcsClient.DestroyLayerCallCount()).To(Equal(1))
			driverInfo, layerId = hcsClient.DestroyLayerArgsForCall(0)
			Expect(driverInfo).To(Equal(expectedDriverInfo))
			Expect(layerId).To(Equal(expectedLayerId))
		})

		Context("when unpreparing the bundlePath fails", func() {
			var unprepareLayerError = errors.New("unprepare sandbox failed")

			BeforeEach(func() {
				hcsClient.UnprepareLayerReturns(unprepareLayerError)
			})

			It("errors", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(unprepareLayerError))
			})
		})

		Context("when deactivating the bundlePath fails", func() {
			var deactivateLayerError = errors.New("deactivate sandbox failed")

			BeforeEach(func() {
				hcsClient.DeactivateLayerReturns(deactivateLayerError)
			})

			It("errors", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(deactivateLayerError))
			})
		})

		Context("when destroying the bundlePath fails", func() {
			var destroyLayerError = errors.New("destroying sandbox failed")

			BeforeEach(func() {
				hcsClient.DestroyLayerReturns(destroyLayerError)
			})

			It("errors", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(destroyLayerError))
			})
		})
	})
})
