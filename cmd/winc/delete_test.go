package main_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo"
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

			bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateSandbox(imageStore, rootfsPath, containerId))
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
		})

		AfterEach(func() {
			helpers.DeleteContainer(containerId)
			helpers.DeleteSandbox(imageStore, containerId)
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
		})
	})

	Context("when provided a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "delete", "nonexistentcontainer")
			stdOut, stdErr, err := helpers.Execute(cmd)
			Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())

			Expect(stdErr.String()).To(ContainSubstring("container not found: nonexistentcontainer"))
		})
	})
})
