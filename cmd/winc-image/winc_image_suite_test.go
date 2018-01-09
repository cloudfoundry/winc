package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	testhelpers "code.cloudfoundry.org/winc/cmd/helpers"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	wincImageBin string
	rootfsPath   string
	helpers      *testhelpers.Helpers
)

func TestWincImage(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		var (
			err     error
			present bool
		)

		rootfsPath, present = os.LookupEnv("WINC_TEST_ROOTFS")
		Expect(present).To(BeTrue(), "WINC_TEST_ROOTFS not set")
		wincImageBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc-image")
		Expect(err).ToNot(HaveOccurred())

		wincImageDir := filepath.Dir(wincImageBin)

		err = exec.Command("gcc.exe", "-c", "..\\..\\image\\volume\\quota\\quota.c", "-o", filepath.Join(wincImageDir, "quota.o")).Run()
		Expect(err).NotTo(HaveOccurred())

		err = exec.Command("gcc.exe",
			"-shared",
			"-o", filepath.Join(wincImageDir, "quota.dll"),
			filepath.Join(wincImageDir, "quota.o"),
			"-lole32", "-loleaut32").Run()
		Expect(err).NotTo(HaveOccurred())

		helpers = testhelpers.NewHelpers("", wincImageBin, "")
	})

	AfterSuite(func() {
		gexec.CleanupBuildArtifacts()
	})

	RunSpecs(t, "WincImage Suite")
}

func getVolumeGuid(storePath, id string) string {
	driverInfo := hcsshim.DriverInfo{
		HomeDir: storePath,
		Flavour: 1,
	}
	volumePath, err := hcsshim.GetLayerMountPath(driverInfo, id)
	Expect(err).NotTo(HaveOccurred())
	return volumePath
}

func mountSandboxVolume(storePath, containerId, mountPath string) {
	volumeGuid := getVolumeGuid(storePath, containerId)
	Expect(exec.Command("mountvol", mountPath, volumeGuid).Run()).To(Succeed())
}

func unmountSandboxVolume(mountPath string) {
	if _, _, err := helpers.Execute(exec.Command("mountvol", mountPath, "/L")); err != nil {
		return
	}

	_, _, err := helpers.Execute(exec.Command("mountvol", mountPath, "/D"))
	Expect(err).NotTo(HaveOccurred())
	Expect(os.RemoveAll(mountPath)).To(Succeed())
}

func createWithDiskLimit(storePath, rootfsPath, containerId string, diskLimitSizeBytes int) {
	createCommand := exec.Command(wincImageBin, "--store", storePath, "create", "--disk-limit-size-bytes", strconv.Itoa(diskLimitSizeBytes), rootfsPath, containerId)
	_, _, err := helpers.Execute(createCommand)
	Expect(err).NotTo(HaveOccurred())
}

type DiskUsage struct {
	TotalBytesUsed     uint64 `json:"total_bytes_used"`
	ExclusiveBytesUsed uint64 `json:"exclusive_bytes_used"`
}

type ImageStats struct {
	Disk DiskUsage `json:"disk_usage"`
}

func getImageStats(storePath, containerId string) ImageStats {
	stdout, _, err := helpers.Execute(exec.Command(wincImageBin, "--store", storePath, "stats", containerId))
	Expect(err).NotTo(HaveOccurred())
	var imageStats ImageStats
	Expect(json.Unmarshal(stdout.Bytes(), &imageStats)).To(Succeed())
	return imageStats
}
