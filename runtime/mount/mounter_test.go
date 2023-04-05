package mount_test

import (
	"crypto/rand"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/winc/runtime/mount"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Mounter", func() {
	var (
		volumeGuid string
		pid        int
		mountPath  string
		logger     *logrus.Entry
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

		logger = (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "state")

		mountPath = filepath.Join("C:\\", "proc", strconv.Itoa(pid), "root")
	})

	AfterEach(func() {
		if err := exec.Command("mountvol", mountPath, "/L").Run(); err == nil {
			_ = exec.Command("mountvol", mountPath, "/D").Run()
		}
	})

	It("mounts and unmounts a volume", func() {
		mounter := &mount.Mounter{}

		Expect(mounter.Mount(pid, volumeGuid, logger)).To(Succeed())
		outBytes, err := exec.Command("mountvol", mountPath, "/L").CombinedOutput()
		Expect(err).ToNot(HaveOccurred())
		Expect(string(outBytes)).To(ContainSubstring(volumeGuid))

		Expect(mounter.Unmount(pid)).To(Succeed())
	})

	It("mount a volume for a pid that already exist", func() {
		mounter := &mount.Mounter{}

		Expect(os.MkdirAll(mountPath, 0755)).To(Succeed())

		err := mounter.Mount(pid, volumeGuid, logger)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(MatchRegexp("^mountdir exists"))
	})
})
