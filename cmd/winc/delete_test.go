package main_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Delete", func() {
	Context("when provided an existing container id", func() {
		var containerId string

		BeforeEach(func() {
			containerId = filepath.Base(bundlePath)

			bundleSpec := runtimeSpecGenerator(createSandbox(rootPath, rootfsPath, containerId))
			config, err := json.Marshal(&bundleSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0666)).To(Succeed())
			_, _, err = execute(exec.Command(wincBin, "create", "-b", bundlePath, containerId))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, _, err := execute(exec.Command(wincImageBin, "--store", rootPath, "delete", containerId))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			_, _, err := execute(exec.Command(wincBin, "delete", containerId))
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the container is running", func() {
			var (
				containerEndpoints []string
				rootPath           string
			)

			BeforeEach(func() {
				containerEndpoints = allEndpoints(containerId)
				pid := getContainerState(containerId).Pid
				rootPath = filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root")
				_, err := os.Lstat(rootPath)
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes the container", func() {
				Expect(containerExists(containerId)).To(BeFalse())
			})

			It("deletes the container endpoints", func() {
				existingEndpoints, err := hcsshim.HNSListEndpointRequest()
				Expect(err).NotTo(HaveOccurred())

				for _, ep := range containerEndpoints {
					for _, existing := range existingEndpoints {
						Expect(ep).NotTo(Equal(existing.Id))
					}
				}
			})

			It("does not delete the bundle directory", func() {
				Expect(bundlePath).To(BeADirectory())
			})

			It("unmounts sandbox.vhdx", func() {
				Expect(rootPath).NotTo(BeADirectory())

				// if not cleanly unmounted, the mount point is left as a symlink
				_, err := os.Lstat(rootPath)
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Context("when provided a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "delete", "nonexistentcontainer")
			stdErr := new(bytes.Buffer)
			session, err := gexec.Start(cmd, GinkgoWriter, io.MultiWriter(stdErr, GinkgoWriter))
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(stdErr.String()).To(ContainSubstring("container not found: nonexistentcontainer"))
		})
	})
})
