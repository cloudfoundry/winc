package main_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/sandbox"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Delete", func() {
	Context("when provided an existing container id", func() {
		var (
			containerId string
			bundlePath  string
			cm          container.ContainerManager
		)

		BeforeEach(func() {
			var err error
			bundlePath, err = ioutil.TempDir("", "winccontainer")
			Expect(err).To(Succeed())

			rootfsPath, present := os.LookupEnv("WINC_TEST_ROOTFS")
			Expect(present).To(BeTrue())
			containerId = filepath.Base(bundlePath)

			client := hcsclient.HCSClient{}
			sm := sandbox.NewManager(&client, bundlePath)
			cm = container.NewManager(&client, sm, containerId)

			Expect(cm.Create(rootfsPath)).To(Succeed())
		})

		Context("when the container is not running", func() {
			It("deletes the container and all its resources", func() {
				cmd := exec.Command(wincBin, "delete", containerId)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0))

				query := hcsshim.ComputeSystemQuery{
					Owners: []string{"winc"},
					IDs:    []string{containerId},
				}
				containers, err := hcsshim.GetContainers(query)
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(HaveLen(0))

				_, err = os.Stat(bundlePath)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		XContext("when the container is running", func() {
			It("does not delete the container", func() {
			})

			Context("when passed the -f flag", func() {
				It("deletes the container", func() {
				})
			})
		})
	})

	Context("when provided a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "delete", "nonexistentcontainer")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &hcsclient.NotFoundError{Id: "nonexistentcontainer"}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))
		})
	})
})
