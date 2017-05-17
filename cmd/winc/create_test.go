package main_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"

	. "code.cloudfoundry.org/winc/cmd/winc"
	"github.com/microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create", func() {
	Context("when provided a unique container id", func() {
		var (
			config      []byte
			bundlePath  string
			containerId string
		)

		BeforeEach(func() {
			var err error
			bundlePath, err = ioutil.TempDir("", "winccontainer")
			Expect(err).NotTo(HaveOccurred())

			rootfsPath, present := os.LookupEnv("WINC_TEST_ROOTFS")
			Expect(present).To(BeTrue())
			containerId = filepath.Base(bundlePath)

			bundleSpec := specGenerator(rootfsPath)
			config, err = json.Marshal(&bundleSpec)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0755)).To(Succeed())
		})

		AfterEach(func() {
			Expect(container.Delete(containerId)).To(Succeed())

			_, err := os.Stat(bundlePath)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("creates a container", func() {
			cmd := exec.Command(wincBin, "create", "-b", bundlePath, containerId)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			query := hcsshim.ComputeSystemQuery{
				Owners: []string{"winc"},
				IDs:    []string{containerId},
			}
			containers, err := hcsshim.GetContainers(query)
			Expect(err).ToNot(HaveOccurred())
			Expect(containers).To(HaveLen(1))
			Expect(containers[0].ID).To(Equal(containerId))
			Expect(containers[0].Name).To(Equal(bundlePath))
		})

		It("uses the current directory as the bundle path if not provided", func() {
			cmd := exec.Command(wincBin, "create", containerId)
			cmd.Dir = bundlePath
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			_, err = os.Stat(filepath.Join(bundlePath, "sandbox.vhdx"))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when provided a non-unique container id", func() {
		It("errors", func() {
			bundlePath, err := ioutil.TempDir("", "winccontainer")
			Expect(err).NotTo(HaveOccurred())

			rootfsPath, present := os.LookupEnv("WINC_TEST_ROOTFS")
			Expect(present).To(BeTrue())
			containerId := filepath.Base(bundlePath)

			bundleSpec := specGenerator(rootfsPath)
			config, err := json.Marshal(&bundleSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0755)).To(Succeed())
			Expect(container.Create(rootfsPath, bundlePath, containerId)).To(Succeed())
			defer container.Delete(containerId)

			cmd := exec.Command(wincBin, "create", "-b", bundlePath, containerId)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &container.ContainerExistsError{Id: containerId}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))
		})
	})

	Context("when provided a nonexistent bundle directory", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "create", "-b", "idontexist", "")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := os.PathError{Op: "GetFileAttributesEx", Path: "idontexist", Err: errors.New("")}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))
		})
	})

	Context("when provided a bundle with a config.json that is invalid JSON", func() {
		var (
			config     []byte
			bundlePath string
		)

		BeforeEach(func() {
			var err error
			bundlePath, err = ioutil.TempDir("", "winccontainer")
			Expect(err).NotTo(HaveOccurred())

			config = []byte("{")
			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0755)).To(Succeed())
		})

		It("errors", func() {
			wincCmd := exec.Command(wincBin, "create", "-b", bundlePath, "")
			session, err := gexec.Start(wincCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &json.SyntaxError{}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))
		})
	})

	Context("when provided a bundle with a config.json that does not conform to the runtime spec", func() {
		It("errors", func() {
			bundlePath, err := ioutil.TempDir("", "winccontainer")
			Expect(err).NotTo(HaveOccurred())

			rootfsPath, present := os.LookupEnv("WINC_TEST_ROOTFS")
			Expect(present).To(BeTrue())
			containerId := filepath.Base(bundlePath)

			bundleSpec := specGenerator(rootfsPath)
			bundleSpec.Platform.OS = ""
			config, err := json.Marshal(&bundleSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0755)).To(Succeed())
			cmd := exec.Command(wincBin, "create", "-b", bundlePath, containerId)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &WincBundleConfigValidationError{}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))
		})
	})
})
