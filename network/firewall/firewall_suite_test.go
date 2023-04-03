package firewall_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestFirewall(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Firewall Suite")
}

var (
	firewallDir string
	firewallDLL string
)

var _ = BeforeSuite(func() {
	var err error
	firewallDir, err = ioutil.TempDir("", "network.firewall")
	Expect(err).NotTo(HaveOccurred())
	firewallDLL = filepath.Join(firewallDir, "firewall.dll")

	o, err := exec.Command("gcc.exe", "-c", ".\\dll\\firewall.c", "-o", filepath.Join(firewallDir, "firewall.o")).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(o))

	err = exec.Command("gcc.exe",
		"-shared",
		"-o", firewallDLL,
		filepath.Join(firewallDir, "firewall.o"),
		"-lole32", "-loleaut32").Run()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(os.RemoveAll(firewallDir)).To(Succeed())
})
