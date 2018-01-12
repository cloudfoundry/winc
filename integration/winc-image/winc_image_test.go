package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("WincImage", func() {
	var (
		storePath   string
		containerId string
	)

	BeforeEach(func() {
		var err error
		containerId = helpers.RandomContainerId()
		storePath, err = ioutil.TempDir("", "container-store")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	It("creates and deletes a sandbox", func() {
		createdSpec := helpers.CreateSandbox(storePath, rootfsPath, containerId)

		volumeGuid := getVolumeGuid(storePath, containerId)
		Expect(createdSpec.Version).To(Equal(specs.Version))
		Expect(createdSpec.Root.Path).To(Equal(volumeGuid))
		Expect(createdSpec.Windows.LayerFolders).ToNot(BeEmpty())
		Expect(createdSpec.Windows.LayerFolders[0]).To(Equal(rootfsPath))
		for _, layer := range createdSpec.Windows.LayerFolders {
			Expect(layer).To(BeADirectory())
		}

		driverInfo := hcsshim.DriverInfo{HomeDir: storePath, Flavour: 1}
		Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeTrue())

		helpers.DeleteSandbox(storePath, containerId)

		Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeFalse())
		Expect(filepath.Join(driverInfo.HomeDir, containerId)).NotTo(BeADirectory())
	})

	Context("when provided --log <log-file>", func() {
		var (
			logFile       string
			tempDir       string
			createCommand *exec.Cmd
		)

		BeforeEach(func() {
			var err error
			tempDir, err = ioutil.TempDir("", "log-dir")
			Expect(err).NotTo(HaveOccurred())

			logFile = filepath.Join(tempDir, "winc-image.log")
		})

		AfterEach(func() {
			helpers.DeleteSandbox(storePath, containerId)
			Expect(os.RemoveAll(tempDir)).To(Succeed())
		})

		Context("when the provided log file path does not exist", func() {
			BeforeEach(func() {
				logFile = filepath.Join(tempDir, "some-dir", "winc-image.log")
				createCommand = exec.Command(wincImageBin, "--log", logFile, "--store", storePath, "create", rootfsPath, containerId)
			})

			It("creates the full path", func() {
				_, _, err := helpers.Execute(createCommand)
				Expect(err).NotTo(HaveOccurred())

				Expect(logFile).To(BeAnExistingFile())
			})
		})

		Context("when it runs successfully", func() {
			BeforeEach(func() {
				createCommand = exec.Command(wincImageBin, "--log", logFile, "--store", storePath, "create", rootfsPath, containerId)
			})

			It("does not log to the specified file", func() {
				_, _, err := helpers.Execute(createCommand)
				Expect(err).NotTo(HaveOccurred())

				contents, err := ioutil.ReadFile(logFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(BeEmpty())
			})

			Context("when provided --debug", func() {
				BeforeEach(func() {
					createCommand = exec.Command(wincImageBin, "--log", logFile, "--debug", "--store", storePath, "create", rootfsPath, containerId)
				})

				It("outputs debug level logs", func() {
					_, _, err := helpers.Execute(createCommand)
					Expect(err).NotTo(HaveOccurred())

					contents, err := ioutil.ReadFile(logFile)
					Expect(err).NotTo(HaveOccurred())

					Expect(string(contents)).NotTo(BeEmpty())
				})
			})
		})

		Context("when it errors", func() {
			BeforeEach(func() {
				createCommand = exec.Command(wincImageBin, "--log", logFile, "--store", storePath, "create", "garbage-something", containerId)
			})

			It("logs errors to the specified file", func() {
				helpers.Execute(createCommand)

				contents, err := ioutil.ReadFile(logFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).NotTo(BeEmpty())
			})
		})
	})

	Context("when using unix style rootfsPath", func() {
		var (
			tempRootfs string
			tempdir    string
			err        error
		)

		BeforeEach(func() {
			tempdir, err = ioutil.TempDir("", "rootfs")
			Expect(err).ToNot(HaveOccurred())
			err := exec.Command("cmd.exe", "/c", fmt.Sprintf("mklink /D %s %s", filepath.Join(tempdir, "rootfs"), rootfsPath)).Run()
			Expect(err).ToNot(HaveOccurred())

			tempRootfs = strings.Replace(tempdir, "C:", "", -1) + "/rootfs"
		})

		AfterEach(func() {
			// remove symlink so we don't clobber rootfs dir
			err := exec.Command("cmd.exe", "/c", fmt.Sprintf("rmdir %s", filepath.Join(tempdir, "rootfs"))).Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(os.RemoveAll(tempdir)).To(Succeed())
		})

		destToWindowsPath := func(input string) string {
			vol := filepath.VolumeName(input)
			if vol == "" {
				input = filepath.Join("C:", input)
			}
			return filepath.Clean(input)
		}

		It("creates and deletes a sandbox with unix rootfsPath", func() {
			createdSpec := helpers.CreateSandbox(storePath, tempRootfs, containerId)

			volumeGuid := getVolumeGuid(storePath, containerId)
			Expect(createdSpec.Version).To(Equal(specs.Version))
			Expect(createdSpec.Root.Path).To(Equal(volumeGuid))
			Expect(createdSpec.Windows.LayerFolders).ToNot(BeEmpty())
			Expect(createdSpec.Windows.LayerFolders[0]).To(Equal(destToWindowsPath(tempRootfs)))
			for _, layer := range createdSpec.Windows.LayerFolders {
				Expect(layer).To(BeADirectory())
			}

			driverInfo := hcsshim.DriverInfo{HomeDir: storePath, Flavour: 1}
			Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeTrue())

			helpers.DeleteSandbox(storePath, containerId)

			Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeFalse())
		})
	})

	Context("when provided a disk limit", func() {
		var (
			mountPath          string
			diskLimitSizeBytes int
		)

		BeforeEach(func() {
			var err error
			mountPath, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			unmountSandboxVolume(mountPath)
			Expect(os.RemoveAll(mountPath)).To(Succeed())
			helpers.DeleteSandbox(storePath, containerId)
		})

		Context("the disk limit is greater than 0", func() {
			BeforeEach(func() {
				diskLimitSizeBytes = 50 * 1024 * 1024
				createWithDiskLimit(storePath, rootfsPath, containerId, diskLimitSizeBytes)
				mountSandboxVolume(storePath, containerId, mountPath)
			})

			It("doesn't allow files large than the limit to be created", func() {
				largeFilePath := filepath.Join(mountPath, "file.txt")
				Expect(exec.Command("fsutil", "file", "createnew", largeFilePath, strconv.Itoa(diskLimitSizeBytes+6*1024)).Run()).ToNot(Succeed())
				Expect(largeFilePath).ToNot(BeAnExistingFile())
			})

			It("allows files at the limit to be created", func() {
				largeFilePath := filepath.Join(mountPath, "file.txt")
				Expect(exec.Command("fsutil", "file", "createnew", largeFilePath, strconv.Itoa(diskLimitSizeBytes)).Run()).To(Succeed())
				Expect(largeFilePath).To(BeAnExistingFile())
			})
		})

		Context("the disk limit is equal to 0", func() {
			BeforeEach(func() {
				diskLimitSizeBytes = 0
				createWithDiskLimit(storePath, rootfsPath, containerId, diskLimitSizeBytes)
				mountSandboxVolume(storePath, containerId, mountPath)
			})

			It("does not set a limit", func() {
				output, err := exec.Command("dirquota", "quota", "list", fmt.Sprintf("/Path:%s", mountPath)).CombinedOutput()
				Expect(err).To(HaveOccurred())
				Expect(string(output)).To(ContainSubstring("The requested object was not found"))
			})
		})

		Context("the disk limit is less then 0", func() {
			BeforeEach(func() {
				diskLimitSizeBytes = -5
			})

			It("errors", func() {
				createCommand := exec.Command(wincImageBin, "--store", storePath, "create", "--disk-limit-size-bytes", strconv.Itoa(diskLimitSizeBytes), rootfsPath, containerId)
				_, _, err := helpers.Execute(createCommand)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("when creating the sandbox layer fails", func() {
		It("errors", func() {
			_, stderr, err := helpers.Execute(exec.Command(wincImageBin, "create", "some-bad-rootfs", ""))
			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(ContainSubstring("rootfs layer does not exist"))
		})
	})

	Context("deleting when provided a nonexistent containerId", func() {
		var logFile string

		BeforeEach(func() {
			logF, err := ioutil.TempFile("", "winc-image.log")
			Expect(err).NotTo(HaveOccurred())
			logFile = logF.Name()
			logF.Close()
		})

		AfterEach(func() {
			Expect(os.Remove(logFile)).To(Succeed())
		})

		It("logs a warning", func() {
			_, _, err := helpers.Execute(exec.Command(wincImageBin, "--log", logFile, "delete", "some-bad-container-id"))
			Expect(err).ToNot(HaveOccurred())

			contents, err := ioutil.ReadFile(logFile)
			Expect(string(contents)).To(ContainSubstring("Layer `some-bad-container-id` not found. Skipping delete."))
		})
	})

	Context("when create is called with the wrong number of args", func() {
		It("prints the usage", func() {
			stdOut, _, err := helpers.Execute(exec.Command(wincImageBin, "create"))
			Expect(err).To(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("Incorrect Usage"))
		})
	})

	Context("when delete is called with the wrong number of args", func() {
		It("prints the usage", func() {
			stdOut, _, err := helpers.Execute(exec.Command(wincImageBin, "delete"))
			Expect(err).To(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("Incorrect Usage"))
		})
	})

	Context("deleting after failed attempts", func() {
		var (
			driverInfo    hcsshim.DriverInfo
			sandboxLayers []string
		)

		BeforeEach(func() {
			driverInfo = hcsshim.DriverInfo{HomeDir: storePath, Flavour: 1}

			parentLayerChain, err := ioutil.ReadFile(filepath.Join(rootfsPath, "layerchain.json"))
			Expect(err).NotTo(HaveOccurred())
			parentLayers := []string{}
			Expect(json.Unmarshal(parentLayerChain, &parentLayers)).To(Succeed())

			sandboxLayers = append([]string{rootfsPath}, parentLayers...)
		})

		Context("when a layer has been created but is not activated", func() {
			It("destroys the layer", func() {
				Expect(hcsshim.CreateSandboxLayer(driverInfo, containerId, rootfsPath, sandboxLayers)).To(Succeed())

				helpers.DeleteSandbox(storePath, containerId)
				Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeFalse())
			})
		})

		Context("when a layer has been created and activated but is not prepared", func() {
			It("destroys the layer", func() {
				Expect(hcsshim.CreateSandboxLayer(driverInfo, containerId, rootfsPath, sandboxLayers)).To(Succeed())
				Expect(hcsshim.ActivateLayer(driverInfo, containerId)).To(Succeed())

				helpers.DeleteSandbox(storePath, containerId)
				Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeFalse())
			})
		})
	})
})
