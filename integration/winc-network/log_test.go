package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logging", func() {
	var (
		logFile string
		tempDir string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "log-dir")
		Expect(err).NotTo(HaveOccurred())

		logFile = filepath.Join(tempDir, "winc-network.log")

		networkConfig = helpers.GenerateNetworkConfig()
	})

	AfterEach(func() {
		failed = failed || CurrentSpecReport().Failed()
		deleteContainerAndNetwork(containerId, networkConfig)
		Expect(os.RemoveAll(tempDir)).To(Succeed())
	})

	Context("when the provided log file path does not exist", func() {
		BeforeEach(func() {
			logFile = filepath.Join(tempDir, "some-dir", "winc-network.log")
		})

		It("creates the full path", func() {
			helpers.CreateNetwork(networkConfig, networkConfigFile, "--log", logFile)

			Expect(logFile).To(BeAnExistingFile())
		})
	})

	Context("when it runs successfully", func() {
		It("does not log to the specified file when creating a network", func() {
			helpers.CreateNetwork(networkConfig, networkConfigFile, "--log", logFile)

			contents, err := ioutil.ReadFile(logFile)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(contents)).To(BeEmpty())
		})

		It("logs any netsh commands to a file", func() {
			helpers.CreateNetwork(networkConfig, networkConfigFile, "--log", logFile)
			bundleSpec := helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
			helpers.RunContainer(bundleSpec, bundlePath, containerId)

			netin := `{"Pid": 123, "Properties": {"ports":"8080"},"netin": [{"host_port": 0, "container_port": 1234}]}`
			args := []string{"--log", logFile, "--action", "up", "--configFile", networkConfigFile, "--handle", containerId}
			cmd := exec.Command(wincNetworkBin, args...)
			cmd.Stdin = strings.NewReader(netin)
			_, _, err := helpers.Execute(cmd)
			Expect(err).NotTo(HaveOccurred())

			contents, err := ioutil.ReadFile(logFile)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(contents)).To(ContainSubstring("netsh http add urlacl url=http://*:8080/ user=Users"))
		})

		Context("when provided --debug", func() {
			It("outputs debug level logs", func() {
				helpers.CreateNetwork(networkConfig, networkConfigFile, "--log", logFile, "--debug")

				contents, err := ioutil.ReadFile(logFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).NotTo(BeEmpty())
			})
		})
	})

	Context("when it errors", func() {
		BeforeEach(func() {
			c, err := json.Marshal(networkConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(networkConfigFile, c, 0644)).To(Succeed())
		})

		It("logs errors to the specified file", func() {
			exec.Command(wincNetworkBin, "--action", "some-invalid-action", "--log", logFile).CombinedOutput()

			contents, err := ioutil.ReadFile(logFile)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(contents)).NotTo(BeEmpty())
			Expect(string(contents)).To(ContainSubstring("some-invalid-action"))
		})
	})
})
