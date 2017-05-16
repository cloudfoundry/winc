package sandbox_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/winc/sandbox"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Sandbox", func() {
	Context("when provided a base image layer", func() {
		var sandboxLayer string
		var containerId string

		BeforeEach(func() {
			var err error
			sandboxLayer, err = ioutil.TempDir("", "sandbox")
			containerId = filepath.Base(sandboxLayer)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {

			Expect(sandbox.Delete(sandboxLayer, containerId)).To(Succeed())

			_, err := os.Stat(sandboxLayer)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("creates a sandbox layer that contains a sandbox.vhdx, initialized, and valid layerchain.json", func() {
			baseImage, present := os.LookupEnv("WINC_TEST_ROOTFS")
			Expect(present).To(BeTrue())

			err := sandbox.Create(baseImage, sandboxLayer, containerId)
			Expect(err).ToNot(HaveOccurred())

			sandboxVHDX := filepath.Join(sandboxLayer, "sandbox.vhdx")
			_, err = os.Stat(sandboxVHDX)
			Expect(err).ToNot(HaveOccurred())

			cmd := exec.Command("powershell", "-Command", "Test-VHD", sandboxVHDX)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session, time.Second*3).Should(gexec.Exit(0))

			sandboxInitialized := filepath.Join(sandboxLayer, "initialized")
			_, err = os.Stat(sandboxInitialized)
			Expect(err).ToNot(HaveOccurred())

			layerChainPath := filepath.Join(sandboxLayer, "layerchain.json")
			_, err = os.Stat(layerChainPath)
			Expect(err).ToNot(HaveOccurred())

			layerChain, err := ioutil.ReadFile(layerChainPath)
			Expect(err).ToNot(HaveOccurred())

			layers := []string{}
			err = json.Unmarshal(layerChain, &layers)
			Expect(err).ToNot(HaveOccurred())

			Expect(layers[0]).To(Equal(baseImage))
		})
	})

	Context("when provided an invalid sandbox directory", func() {
		It("errors", func() {
			err := sandbox.Create("", "nonexistentsandbox", "")
			Expect(err).To(HaveOccurred())
		})
	})
})
