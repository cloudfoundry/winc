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
	"time"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		RootFS       string   `json:"rootfs,omitempty"`
		LayerFolders []string `json:"layerFolders,omitempty"`
	}

	It("creates and deletes a sandbox", func() {
		stdout, _, err := execute(wincImageBin, "--store", storePath, "create", rootfsPath, containerId)
		Expect(err).NotTo(HaveOccurred())

		var desiredImageSpec DesiredImageSpec
		Expect(json.Unmarshal(stdout.Bytes(), &desiredImageSpec)).To(Succeed())
		Expect(desiredImageSpec.RootFS).To(Equal(getVolumeGuid(storePath, containerId)))
		Expect(desiredImageSpec.LayerFolders).ToNot(BeEmpty())
		Expect(desiredImageSpec.LayerFolders[0]).To(Equal(rootfsPath))
		for _, layer := range desiredImageSpec.LayerFolders {
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
			err = exec.Command("cmd.exe", "/c", fmt.Sprintf("mklink /D %s %s", filepath.Join(tempdir, "rootfs"), rootfsPath)).Run()
			Expect(err).ToNot(HaveOccurred())

			tempRootfs = tempdir + "/rootfs"
		})

		AfterEach(func() {
			// remove symlink so we don't clobber rootfs dir
			err := exec.Command("cmd.exe", "/c", fmt.Sprintf("rmdir %s", filepath.Join(tempdir, "rootfs"))).Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(os.RemoveAll(tempdir)).To(Succeed())
		})

		It("creates and deletes a sandbox with unix rootfsPath", func() {
			stdout, _, err := execute(wincImageBin, "--store", storePath, "create", tempRootfs, containerId)
			Expect(err).NotTo(HaveOccurred())

			var desiredImageSpec DesiredImageSpec
			Expect(json.Unmarshal(stdout.Bytes(), &desiredImageSpec)).To(Succeed())
			Expect(desiredImageSpec.RootFS).To(Equal(getVolumeGuid(storePath, containerId)))
			Expect(desiredImageSpec.LayerFolders).ToNot(BeEmpty())
			Expect(desiredImageSpec.LayerFolders[0]).To(Equal(filepath.Clean(tempRootfs)))
			for _, layer := range desiredImageSpec.LayerFolders {
				Expect(layer).To(BeADirectory())
			}

			driverInfo := hcsshim.DriverInfo{HomeDir: storePath, Flavour: 1}
			Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeTrue())

			Expect(exec.Command(wincImageBin, "--store", storePath, "delete", containerId).Run()).To(Succeed())

			Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeFalse())
		})
	})

	Context("when creating the sandbox layer fails", func() {
		It("errors", func() {
			_, stderr, err := execute(wincImageBin, "create", "some-bad-rootfs", "")
			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(ContainSubstring("rootfs layer does not exist"))
		})
	})

	Context("when provided a nonexistent containerId", func() {
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
