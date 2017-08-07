package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/volume"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Delete", func() {
	var (
		stdOut *bytes.Buffer
		stdErr *bytes.Buffer
	)

	BeforeEach(func() {
		stdOut = new(bytes.Buffer)
		stdErr = new(bytes.Buffer)
	})

	Context("when provided an existing container id", func() {
		var (
			containerId string
			cm          *container.Manager
		)

		BeforeEach(func() {
			containerId = filepath.Base(bundlePath)

			client := hcs.Client{}
			nm := networkManager(&client)
			cm = container.NewManager(&client, &volume.Mounter{}, nm, rootPath, bundlePath)

			bundleSpec := runtimeSpecGenerator(createSandbox(rootPath, rootfsPath, containerId), containerId)
			Expect(cm.Create(&bundleSpec)).To(Succeed())
		})

		AfterEach(func() {
			Expect(execute(wincImageBin, "--store", rootPath, "delete", containerId)).To(Succeed())
		})

		Context("when the container is running", func() {
			It("deletes the container", func() {
				err := execute(wincBin, "delete", containerId)
				Expect(err).ToNot(HaveOccurred())

				Expect(containerExists(containerId)).To(BeFalse())
			})

			It("deletes the container endpoints", func() {
				containerEndpoints := allEndpoints(containerId)

				err := execute(wincBin, "delete", containerId)
				Expect(err).ToNot(HaveOccurred())

				existingEndpoints, err := hcsshim.HNSListEndpointRequest()
				Expect(err).NotTo(HaveOccurred())

				for _, ep := range containerEndpoints {
					for _, existing := range existingEndpoints {
						Expect(ep).NotTo(Equal(existing.Id))
					}
				}
			})

			It("does not delete the bundle directory", func() {
				err := execute(wincBin, "delete", containerId)
				Expect(err).ToNot(HaveOccurred())

				Expect(bundlePath).To(BeADirectory())
			})

			It("unmounts sandbox.vhdx", func() {
				state, err := cm.State()
				Expect(err).NotTo(HaveOccurred())
				rootPath := filepath.Join("c:\\", "proc", strconv.Itoa(state.Pid), "root")
				_, err = os.Lstat(rootPath)
				Expect(err).NotTo(HaveOccurred())

				err = execute(wincBin, "delete", containerId)
				Expect(err).ToNot(HaveOccurred())

				Expect(rootPath).NotTo(BeADirectory())

				// if not cleanly unmounted, the mount point is left as a symlink
				_, err = os.Lstat(rootPath)
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Context("when provided a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "delete", "nonexistentcontainer")
			session, err := gexec.Start(cmd, stdOut, stdErr)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &hcs.NotFoundError{Id: "nonexistentcontainer"}
			Expect(stdErr.String()).To(ContainSubstring(expectedError.Error()))
		})
	})
})
