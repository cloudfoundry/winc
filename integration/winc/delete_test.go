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

			Context("when passed the -force flag", func() {
				It("deletes the container", func() {
					cmd := exec.Command(wincBin, "delete", "-force", containerId)
					stdOut, stdErr, err := helpers.Execute(cmd)
					Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
					Expect(helpers.ContainerExists(containerId)).To(BeFalse())
				})
			})

			FContext("when there is another container (i.e., a pea) that shares a network with the main container", func() {
				var (
					peaBundlePath, peaContainerId string
				)

				BeforeEach(func() {
					var err error
					peaBundlePath, err = ioutil.TempDir("", "winccontainerpea")
					Expect(err).NotTo(HaveOccurred())
					peaContainerId = filepath.Base(peaBundlePath)
					peaBundleSpec := helpers.GenerateRuntimeSpec(helpers.CreateSandbox(imageStore, rootfsPath, peaContainerId))
					// peaBundleSpec.Windows.Network = &specs.WindowsNetwork{NetworkSharedContainerName: containerId} WHY DO I CAUSE THE TESTS TO FAIL
					helpers.CreateContainer(peaBundleSpec, peaBundlePath, peaContainerId)
				})

				AfterEach(func() {
					helpers.DeleteContainer(peaContainerId)
					helpers.DeleteSandbox(imageStore, peaContainerId)
					Expect(os.RemoveAll(peaBundlePath)).To(Succeed())
				})

				It("deletes the main container and the pea container", func() {
					helpers.DeleteContainer(containerId)
					Expect(helpers.ContainerExists(containerId)).To(BeFalse())
					Expect(helpers.ContainerExists(peaContainerId)).To(BeFalse())
				})

				It("does not delete the bundle directories", func() {
					helpers.DeleteContainer(containerId)
					Expect(bundlePath).To(BeADirectory())
					Expect(peaBundlePath).To(BeADirectory())
				})

				It("unmounts sandbox.vhdx", func() {
					pid := helpers.GetContainerState(containerId).Pid
					peaPid := helpers.GetContainerState(peaContainerId).Pid
					helpers.DeleteContainer(containerId)
					rootPath := filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root")
					Expect(rootPath).NotTo(BeADirectory())
					peaRootPath := filepath.Join("c:\\", "proc", strconv.Itoa(peaPid), "root")
					Expect(peaRootPath).NotTo(BeADirectory())

					// if not cleanly unmounted, the mount point is left as a symlink
					_, err := os.Lstat(rootPath)
					Expect(err).NotTo(BeNil())
					_, err = os.Lstat(peaRootPath)
					Expect(err).NotTo(BeNil())
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
