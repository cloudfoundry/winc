package mount_test

import (
	"crypto/rand"
	"math"
	"math/big"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/winc/container/mount"
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

		max := big.NewInt(math.MaxInt32)
		p, err := rand.Int(rand.Reader, max)
		Expect(err).NotTo(HaveOccurred())
		// negate so we don't collide with any 'real' pids created by winc
		pid = -int(p.Int64())

		mountPath = filepath.Join("C:\\", "proc", strconv.Itoa(pid), "root")
	})

	AfterEach(func() {
		if err := exec.Command("mountvol", mountPath, "/L").Run(); err == nil {
			_ = exec.Command("mountvol", mountPath, "/D").Run()
		}
	})

	It("mounts and unmounts a volume", func() {
		mounter := &mount.Mounter{}

		Expect(mounter.Mount(pid, volumeGuid)).To(Succeed())
		outBytes, err := exec.Command("mountvol", mountPath, "/L").CombinedOutput()
		Expect(err).ToNot(HaveOccurred())
		Expect(string(outBytes)).To(ContainSubstring(volumeGuid))

		Expect(mounter.Unmount(pid)).To(Succeed())
	})
})
