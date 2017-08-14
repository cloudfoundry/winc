package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	wincImageBin string
	rootfsPath   string
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

		err = exec.Command("gcc.exe", "-c", "..\\..\\volume\\quota\\quota.c", "-o", filepath.Join(wincImageDir, "quota.o")).Run()
		Expect(err).NotTo(HaveOccurred())

		err = exec.Command("gcc.exe",
			"-shared",
			"-o", filepath.Join(wincImageDir, "quota.dll"),
			filepath.Join(wincImageDir, "quota.o"),
			"-lole32", "-loleaut32").Run()
		Expect(err).NotTo(HaveOccurred())
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

func execute(cmd string, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	command := exec.Command(cmd, args...)
	command.Stdout = stdOut
	command.Stderr = stdErr
	err := command.Run()
	return stdOut, stdErr, err
}
