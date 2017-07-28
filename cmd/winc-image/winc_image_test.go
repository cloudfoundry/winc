package main_test

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"os/exec"
	"strconv"
	"time"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const depotDir = `C:\var\vcap\data\winc-image\depot`

var _ = Describe("WincImage", func() {
	var (
		containerId string
	)

	BeforeEach(func() {
		rand.Seed(time.Now().UnixNano())
		containerId = strconv.Itoa(rand.Int())
	})

	type DesiredImageSpec struct {
		RootFS string `json:"rootfs,omitempty"`
		Image  struct {
			Config struct {
				Layers []string `json:"layers,omitempty"`
			} `json:"config,omitempty"`
		} `json:"image,omitempty"`
	}

	It("creates and deletes a sandbox", func() {
		stdout, _, err := execute(wincImageBin, "create", rootfsPath, containerId)
		Expect(err).NotTo(HaveOccurred())

		var desiredImageSpec DesiredImageSpec
		Expect(json.Unmarshal(stdout.Bytes(), &desiredImageSpec)).To(Succeed())
		Expect(desiredImageSpec.RootFS).To(Equal(getVolumeGuid(depotDir, containerId)))
		Expect(desiredImageSpec.Image.Config.Layers).ToNot(BeEmpty())
		Expect(desiredImageSpec.Image.Config.Layers[0]).To(Equal(rootfsPath))
		for _, layer := range desiredImageSpec.Image.Config.Layers {
			Expect(layer).To(BeADirectory())
		}

		driverInfo := hcsshim.DriverInfo{HomeDir: depotDir, Flavour: 1}
		Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeTrue())

		Expect(exec.Command(wincImageBin, "delete", containerId).Run()).To(Succeed())

		Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeFalse())
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

func getVolumeGuid(depotDir, id string) string {
	driverInfo := hcsshim.DriverInfo{
		HomeDir: depotDir,
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
