package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
		rand.Seed(time.Now().UnixNano())
		containerId = strconv.Itoa(rand.Int())
		storePath, err = ioutil.TempDir("", "container-store")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	type DesiredImageSpec struct {
		RootFS string `json:"rootfs,omitempty"`
		specs.Spec
	}

	It("creates and deletes a sandbox", func() {
		stdout, _, err := execute(wincImageBin, "--store", storePath, "create", rootfsPath, containerId)
		Expect(err).NotTo(HaveOccurred())

		var desiredImageSpec DesiredImageSpec
		Expect(json.Unmarshal(stdout.Bytes(), &desiredImageSpec)).To(Succeed())
		volumeGuid := getVolumeGuid(storePath, containerId)
		Expect(desiredImageSpec.RootFS).To(Equal(volumeGuid))
		Expect(desiredImageSpec.Root.Path).To(Equal(volumeGuid))
		Expect(desiredImageSpec.Windows.LayerFolders).ToNot(BeEmpty())
		Expect(desiredImageSpec.Windows.LayerFolders[0]).To(Equal(rootfsPath))
		for _, layer := range desiredImageSpec.Windows.LayerFolders {
			Expect(layer).To(BeADirectory())
		}

		driverInfo := hcsshim.DriverInfo{HomeDir: storePath, Flavour: 1}
		Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeTrue())

		Expect(exec.Command(wincImageBin, "--store", storePath, "delete", containerId).Run()).To(Succeed())

		Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeFalse())
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
			stdout, _, err := execute(wincImageBin, "--store", storePath, "create", tempRootfs, containerId)
			Expect(err).NotTo(HaveOccurred())

			var desiredImageSpec DesiredImageSpec
			Expect(json.Unmarshal(stdout.Bytes(), &desiredImageSpec)).To(Succeed())
			volumeGuid := getVolumeGuid(storePath, containerId)
			Expect(desiredImageSpec.RootFS).To(Equal(volumeGuid))
			Expect(desiredImageSpec.Root.Path).To(Equal(volumeGuid))
			Expect(desiredImageSpec.Windows.LayerFolders).ToNot(BeEmpty())
			Expect(desiredImageSpec.Windows.LayerFolders[0]).To(Equal(destToWindowsPath(tempRootfs)))
			for _, layer := range desiredImageSpec.Windows.LayerFolders {
				Expect(layer).To(BeADirectory())
			}

			driverInfo := hcsshim.DriverInfo{HomeDir: storePath, Flavour: 1}
			Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeTrue())

			Expect(exec.Command(wincImageBin, "--store", storePath, "delete", containerId).Run()).To(Succeed())

			Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeFalse())
		})
	})

	Context("when provided a disk limit", func() {
		Context("when the disk limit is valid", func() {
			var (
				mountPath          string
				diskLimitSizeBytes int
				volumeGuid         string
			)

			BeforeEach(func() {
				diskLimitSizeBytes = 50 * 1024 * 1024
			})

			JustBeforeEach(func() {
				_, _, err := execute(wincImageBin, "--store", storePath, "create", "--disk-limit-size-bytes", strconv.Itoa(diskLimitSizeBytes), rootfsPath, containerId)
				Expect(err).NotTo(HaveOccurred())

				mountPath, err = ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())

				volumeGuid = getVolumeGuid(storePath, containerId)
				Expect(exec.Command("mountvol", mountPath, volumeGuid).Run()).To(Succeed())
			})

			AfterEach(func() {
				Expect(exec.Command("mountvol", mountPath, "/D").Run()).To(Succeed())
				Expect(os.RemoveAll(mountPath)).To(Succeed())
				Expect(exec.Command(wincImageBin, "--store", storePath, "delete", containerId).Run()).To(Succeed())
			})

			It("doesn't allow files large than the limit to be created", func() {
				largeFilePath := filepath.Join(mountPath, "file.txt")
				Expect(exec.Command("fsutil", "file", "createnew", largeFilePath, strconv.Itoa(diskLimitSizeBytes+1)).Run()).ToNot(Succeed())
				Expect(largeFilePath).ToNot(BeAnExistingFile())
			})

			It("allows files at the limit to be created", func() {
				largeFilePath := filepath.Join(mountPath, "file.txt")
				Expect(exec.Command("fsutil", "file", "createnew", largeFilePath, strconv.Itoa(diskLimitSizeBytes)).Run()).To(Succeed())
				Expect(largeFilePath).To(BeAnExistingFile())
			})

			Context("when the provided disk limit is 0", func() {
				BeforeEach(func() {
					diskLimitSizeBytes = 0
				})

				It("does not set a limit", func() {
					output, err := exec.Command("powershell.exe", "-Command", "Get-FSRMQuota", mountPath).CombinedOutput()
					Expect(err).To(HaveOccurred())
					Expect(string(output)).To(ContainSubstring("The requested object was not found"))
				})
			})
		})

		Context("when the provided disk limit is below 0", func() {
			It("errors", func() {
				_, _, err := execute(wincImageBin, "--store", storePath, "create", "--disk-limit-size-bytes", "-5", rootfsPath, containerId)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("when creating the sandbox layer fails", func() {
		It("errors", func() {
			_, stderr, err := execute(wincImageBin, "create", "some-bad-rootfs", "")
			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(ContainSubstring("rootfs layer does not exist"))
		})
	})

	Context("deleting when provided a nonexistent containerId", func() {
		It("logs a warning", func() {
			stdOut, _, err := execute(wincImageBin, "delete", "some-bad-container-id")
			Expect(err).ToNot(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("Layer `some-bad-container-id` not found. Skipping delete."))
		})
	})

	Context("when create is called with the wrong number of args", func() {
		It("prints the usage", func() {
			stdOut, _, err := execute(wincImageBin, "create")
			Expect(err).To(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("Incorrect Usage"))
		})
	})

	Context("when delete is called with the wrong number of args", func() {
		It("prints the usage", func() {
			stdOut, _, err := execute(wincImageBin, "delete")
			Expect(err).To(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("Incorrect Usage"))
		})
	})
})

func getVolumeGuid(storePath, id string) string {
	driverInfo := hcsshim.DriverInfo{
		HomeDir: storePath,
		Flavour: 1,
	}
	volumePath, err := hcsshim.GetLayerMountPath(driverInfo, id)
	Expect(err).NotTo(HaveOccurred())
	return volumePath
}

func execute(cmd string, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	command := exec.Command(cmd, args...)
	command.Stdout = stdOut
	command.Stderr = stdErr
	err := command.Run()
	return stdOut, stdErr, err
}
