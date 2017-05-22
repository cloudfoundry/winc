package main_test

import (
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
			cm          container.ContainerManager
		)

		BeforeEach(func() {
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
