package volume_test

import (
	"math/rand"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/winc/volume"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mounter", func() {
	var (
		volumeGuid string
		pid        int
		mountPath  string
	)

	BeforeEach(func() {
		outBytes, err := exec.Command("mountvol", "C:\\", "/L").CombinedOutput()
		Expect(err).ToNot(HaveOccurred())
		volumeGuid = strings.TrimSpace(string(outBytes))

		rand.Seed(time.Now().UnixNano())
		pid = rand.Int()
		mountPath = filepath.Join("C:\\", "proc", strconv.Itoa(pid), "root")
	})

	AfterEach(func() {
		if err := exec.Command("mountvol", mountPath, "/L").Run(); err == nil {
			_ = exec.Command("mountvol", mountPath, "/D").Run()
		}
	})

	It("mounts and unmounts a volume", func() {
		mounter := &volume.Mounter{}

		Expect(mounter.Mount(pid, volumeGuid)).To(Succeed())
		outBytes, err := exec.Command("mountvol", mountPath, "/L").CombinedOutput()
		Expect(err).ToNot(HaveOccurred())
		Expect(string(outBytes)).To(ContainSubstring(volumeGuid))

		Expect(mounter.Unmount(pid)).To(Succeed())
	})
})
