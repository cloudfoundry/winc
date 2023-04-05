package main_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Delete", func() {
	Context("when provided an existing container id", func() {
		var (
			containerId string
			bundlePath  string
			bundleSpec  specs.Spec
		)

		BeforeEach(func() {
			var err error
			bundlePath, err = ioutil.TempDir("", "winccontainer")
			Expect(err).To(Succeed())

			containerId = filepath.Base(bundlePath)

			bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
		})

		AfterEach(func() {
			failed = failed || CurrentSpecReport().Failed()
			helpers.DeleteContainer(containerId)
			helpers.DeleteVolume(containerId)
			Expect(os.RemoveAll(bundlePath)).To(Succeed())
		})

		Context("when the container is running", func() {
			It("deletes the container", func() {
				helpers.DeleteContainer(containerId)
				Expect(helpers.ContainerExists(containerId)).To(BeFalse())
			})

			It("does not delete the bundle directory", func() {
				helpers.DeleteContainer(containerId)
				Expect(bundlePath).To(BeADirectory())
			})

			It("unmounts sandbox.vhdx", func() {
				pid := helpers.GetContainerState(containerId).Pid
				helpers.DeleteContainer(containerId)
				rootPath := filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root")
				Expect(rootPath).NotTo(BeADirectory())

				// if not cleanly unmounted, the mount point is left as a symlink
				_, err := os.Lstat(rootPath)
				Expect(err).NotTo(BeNil())
			})

			Context("when passed the -force flag", func() {
				It("deletes the container", func() {
					cmd := exec.Command(wincBin, "delete", "-force", containerId)
					stdOut, stdErr, err := helpers.Execute(cmd)
					Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
					Expect(helpers.ContainerExists(containerId)).To(BeFalse())
				})
			})
		})
	})

	Context("when provided a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "delete", "nonexistentcontainer")
			stdOut, stdErr, err := helpers.Execute(cmd)
			Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())

			Expect(stdErr.String()).To(ContainSubstring("container not found: nonexistentcontainer"))
		})

		Context("when passed the -force flag", func() {
			It("does not error", func() {
				cmd := exec.Command(wincBin, "delete", "-force", "nonexistentcontainer")
				stdOut, stdErr, err := helpers.Execute(cmd)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
			})
		})
	})
})
