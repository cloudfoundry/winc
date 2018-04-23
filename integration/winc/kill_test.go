package main_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Kill", func() {
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
			helpers.DeleteContainer(containerId)
			helpers.DeleteVolume(containerId)
			Expect(os.RemoveAll(bundlePath)).To(Succeed())
		})

		FIt("kills the process", func() {
			containerPid := helpers.GetContainerState(containerId).Pid
			shutdownHandleBinInContainer := filepath.Join("c:\\", "proc", strconv.Itoa(containerPid), "root", "shutdown-handler.exe")
			helpers.CopyFile(shutdownHandleBinInContainer, shutdownHandlerBin)

			var stdout *gbytes.Buffer

			go func() {
				defer GinkgoRecover()
				stdout, _, _ = helpers.ExecInContainerGbytes(containerId, []string{"C:\\shutdown-handler.exe"}, false)
			}()

			time.Sleep(time.Second * 3)
			// helpers.DeleteContainer(containerId)
			helpers.KillContainer(containerId, "sigkill")
			Eventually(stdout).Should(gbytes.Say("shutdown event received"))
		})

		// It("does not delete the bundle directory", func() {
		// 	helpers.DeleteContainer(containerId)
		// 	Expect(bundlePath).To(BeADirectory())
		// })
		//
		// It("unmounts sandbox.vhdx", func() {
		// 	pid := helpers.GetContainerState(containerId).Pid
		// 	helpers.DeleteContainer(containerId)
		// 	rootPath := filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root")
		// 	Expect(rootPath).NotTo(BeADirectory())
		//
		// 	// if not cleanly unmounted, the mount point is left as a symlink
		// 	_, err := os.Lstat(rootPath)
		// 	Expect(err).NotTo(BeNil())
		// })
		//
		// Context("when passed the -force flag", func() {
		// 	It("deletes the container", func() {
		// 		cmd := exec.Command(wincBin, "delete", "-force", containerId)
		// 		stdOut, stdErr, err := helpers.Execute(cmd)
		// 		Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
		// 		Expect(helpers.ContainerExists(containerId)).To(BeFalse())
		// 	})
		// })
	})

	// Context("when provided a nonexistent container id", func() {
	// 	It("errors", func() {
	// 		cmd := exec.Command(wincBin, "delete", "nonexistentcontainer")
	// 		stdOut, stdErr, err := helpers.Execute(cmd)
	// 		Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())
	//
	// 		Expect(stdErr.String()).To(ContainSubstring("container not found: nonexistentcontainer"))
	// 	})
	//
	// 	Context("when passed the -force flag", func() {
	// 		It("does not error", func() {
	// 			cmd := exec.Command(wincBin, "delete", "-force", "nonexistentcontainer")
	// 			stdOut, stdErr, err := helpers.Execute(cmd)
	// 			Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
	// 		})
	// 	})
	// })
})
