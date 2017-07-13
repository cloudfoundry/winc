package sandbox_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/hcsclient/hcsclientfakes"
	"code.cloudfoundry.org/winc/sandbox"
	"code.cloudfoundry.org/winc/sandbox/sandboxfakes"
	"github.com/Microsoft/hcsshim"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sandbox", func() {
	const containerVolume = "containerVolume"

	var (
		bundlePath         string
		rootfs             string
		hcsClient          *hcsclientfakes.FakeClient
		sandboxManager     sandbox.SandboxManager
		expectedDriverInfo hcsshim.DriverInfo
		expectedLayerId    string
		rootfsParents      []byte
		fakeCommand        *sandboxfakes.FakeCommand
	)

	BeforeEach(func() {
		var err error
		rootfs, err = ioutil.TempDir("", "rootfs")
		Expect(err).ToNot(HaveOccurred())

		bundlePath, err = ioutil.TempDir("", "sandbox")
		Expect(err).ToNot(HaveOccurred())

		hcsClient = &hcsclientfakes.FakeClient{}
		fakeCommand = &sandboxfakes.FakeCommand{}
		sandboxManager = sandbox.NewManager(hcsClient, fakeCommand, bundlePath)

		expectedDriverInfo = hcsshim.DriverInfo{
			HomeDir: filepath.Dir(bundlePath),
			Flavour: 1,
		}
		expectedLayerId = filepath.Base(bundlePath)
		rootfsParents = []byte(`["path1", "path2"]`)

		hcsClient.GetLayerMountPathReturns(containerVolume, nil)
	})

	JustBeforeEach(func() {
		Expect(ioutil.WriteFile(filepath.Join(rootfs, "layerchain.json"), rootfsParents, 0755)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
		Expect(os.RemoveAll(rootfs)).To(Succeed())
	})

	Context("Create", func() {
		Context("when provided a rootfs layer", func() {
			It("creates and activates the bundlePath", func() {
				cv, err := sandboxManager.Create(rootfs)
				Expect(err).ToNot(HaveOccurred())
				Expect(cv).To(Equal(containerVolume))

				expectedLayers := []string{rootfs, "path1", "path2"}

				Expect(hcsClient.CreateSandboxLayerCallCount()).To(Equal(1))
				driverInfo, layerId, parentLayer, parentLayers := hcsClient.CreateSandboxLayerArgsForCall(0)
				Expect(driverInfo).To(Equal(expectedDriverInfo))
				Expect(layerId).To(Equal(expectedLayerId))
				Expect(parentLayer).To(Equal(rootfs))
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
					_, err := sandboxManager.Create(rootfs)
					Expect(err).To(Equal(createSandboxLayerError))
				})
			})

			Context("when activating the bundlePath fails", func() {
				var activateLayerError = errors.New("activate sandbox failed")

				BeforeEach(func() {
					hcsClient.ActivateLayerReturns(activateLayerError)
				})

				It("errors", func() {
					_, err := sandboxManager.Create(rootfs)
					Expect(err).To(Equal(activateLayerError))
				})
			})

			Context("when preparing the bundlePath fails", func() {
				var prepareLayerError = errors.New("prepare sandbox failed")

				BeforeEach(func() {
					hcsClient.PrepareLayerReturns(prepareLayerError)
				})

				It("errors", func() {
					_, err := sandboxManager.Create(rootfs)
					Expect(err).To(Equal(prepareLayerError))
				})
			})
		})

		Context("when provided a nonexistent rootfs layer", func() {
			It("errors", func() {
				_, err := sandboxManager.Create("nonexistentrootfs")
				Expect(err).To(Equal(&sandbox.MissingRootfsError{Msg: "nonexistentrootfs"}))
			})
		})

		Context("when provided a rootfs layer missing a layerchain.json", func() {
			JustBeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(rootfs, "layerchain.json"))).To(Succeed())
			})

			It("errors", func() {
				_, err := sandboxManager.Create(rootfs)
				Expect(err).To(Equal(&sandbox.MissingRootfsLayerChainError{Msg: rootfs}))
			})
		})

		Context("when the rootfs has a layerchain.json that is invalid JSON", func() {
			BeforeEach(func() {
				rootfsParents = []byte("[")
			})

			It("errors", func() {
				_, err := sandboxManager.Create(rootfs)
				Expect(err).To(Equal(&sandbox.InvalidRootfsLayerChainError{Msg: rootfs}))
			})
		})

		Context("when the bundlePath directory does not exist", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(bundlePath)).To(Succeed())
			})

			It("errors", func() {
				_, err := sandboxManager.Create(rootfs)
				Expect(err).To(Equal(&sandbox.MissingBundlePathError{Msg: bundlePath}))
			})
		})

		Context("when getting the volume mount path of the container fails", func() {
			Context("when getting the volume returned an error", func() {
				var layerMountPathError = errors.New("could not get volume")

				BeforeEach(func() {
					hcsClient.GetLayerMountPathReturns("", layerMountPathError)
				})

				It("errors", func() {
					_, err := sandboxManager.Create(rootfs)
					Expect(err).To(Equal(layerMountPathError))
				})
			})

			Context("when the volume returned is empty", func() {
				BeforeEach(func() {
					hcsClient.GetLayerMountPathReturns("", nil)
				})

				It("errors", func() {
					_, err := sandboxManager.Create(rootfs)
					Expect(err).To(Equal(&hcsclient.MissingVolumePathError{Id: expectedLayerId}))
				})
			})
		})
	})

	Context("Delete", func() {
		It("unprepares and deactivates the bundlePath", func() {
			err := sandboxManager.Delete()
			Expect(err).ToNot(HaveOccurred())

			Expect(hcsClient.UnprepareLayerCallCount()).To(Equal(1))
			driverInfo, layerId := hcsClient.UnprepareLayerArgsForCall(0)
			Expect(driverInfo).To(Equal(expectedDriverInfo))
			Expect(layerId).To(Equal(expectedLayerId))

			Expect(hcsClient.DeactivateLayerCallCount()).To(Equal(1))
			driverInfo, layerId = hcsClient.DeactivateLayerArgsForCall(0)
			Expect(driverInfo).To(Equal(expectedDriverInfo))
			Expect(layerId).To(Equal(expectedLayerId))
		})

		It("only deletes the files that the container created", func() {
			sentinelPath := filepath.Join(bundlePath, "sentinel")
			f, err := os.Create(sentinelPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.Close()).To(Succeed())

			err = sandboxManager.Delete()
			Expect(err).ToNot(HaveOccurred())

			files, err := filepath.Glob(filepath.Join(bundlePath, "*"))
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(ConsistOf([]string{filepath.Join(bundlePath, "sentinel")}))
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
	})

	Context("Mount", func() {
		It("mounts the sandbox.vhdx at C:\\proc\\{{pid}}\\root", func() {
			_, err := sandboxManager.Create(rootfs)
			Expect(err).ToNot(HaveOccurred())

			pid := rand.Int()
			Expect(sandboxManager.Mount(pid)).To(Succeed())

			rootPath := filepath.Join("c:\\", "proc", fmt.Sprintf("%d", pid), "root")
			Expect(rootPath).To(BeADirectory())

			Expect(fakeCommand.RunCallCount()).To(Equal(1))
			runCmd, runArgs := fakeCommand.RunArgsForCall(0)
			Expect(runCmd).To(Equal("mountvol"))
			Expect(runArgs[0]).To(Equal(rootPath))
			Expect(runArgs[1]).To(Equal(containerVolume))
		})
	})

	Context("Unmount", func() {
		var pid int
		var mountPath string
		var rootPath string

		BeforeEach(func() {
			pid = rand.Int()
			mountPath = filepath.Join("c:\\", "proc", fmt.Sprintf("%d", pid))
			rootPath = filepath.Join(mountPath, "root")
			Expect(os.MkdirAll(rootPath, 0755)).To(Succeed())
		})

		It("unmounts the sandbox.vhdx from c:\\proc\\<pid>\\mnt and removes the directory", func() {
			Expect(sandboxManager.Unmount(pid)).To(Succeed())

			Expect(fakeCommand.RunCallCount()).To(Equal(1))
			runCmd, runArgs := fakeCommand.RunArgsForCall(0)
			Expect(runCmd).To(Equal("mountvol"))
			Expect(runArgs[0]).To(Equal(rootPath))
			Expect(runArgs[1]).To(Equal("/D"))

			Expect(mountPath).NotTo(BeADirectory())
		})
	})
})
