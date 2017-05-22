package main_test

import (
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/sandbox"
	"github.com/Microsoft/hcsshim"
	ps "github.com/mitchellh/go-ps"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Exec", func() {
	var (
		containerId string
		cm          container.ContainerManager
		client      hcsclient.HCSClient
		commandArgs []string
	)

	containerProcesses := func(containerId, filter string) []hcsshim.ProcessListItem {
		container, err := client.OpenContainer(containerId)
		Expect(err).To(Succeed())

		pl, err := container.ProcessList()
		Expect(err).To(Succeed())

		if filter != "" {
			var filteredPL []hcsshim.ProcessListItem
			for _, v := range pl {
				if v.ImageName == filter {
					filteredPL = append(filteredPL, v)
				}
			}

			return filteredPL
		}

		return pl
	}

	isParentOf := func(parentPid, childPid int) bool {
		var (
			process ps.Process
			err     error
		)

		var foundParent bool
		for {
			process, err = ps.FindProcess(childPid)
			Expect(err).To(Succeed())

			if process == nil {
				break
			}
			if process.PPid() == parentPid {
				foundParent = true
				break
			}
			childPid = process.PPid()
		}

		return foundParent
	}

	BeforeEach(func() {
		containerId = filepath.Base(bundlePath)

		client = hcsclient.HCSClient{}
		sm := sandbox.NewManager(&client, bundlePath)
		cm = container.NewManager(&client, sm, containerId)

		commandArgs = []string{"exec", containerId}
	})

	Context("when the container exists", func() {
		BeforeEach(func() {
			Expect(cm.Create(rootfsPath)).To(Succeed())
		})

		AfterEach(func() {
			Expect(cm.Delete()).To(Succeed())
		})

		Context("when a command is executed in the container", func() {
			BeforeEach(func() {
				pl := containerProcesses(containerId, "powershell.exe")
				Expect(pl).To(BeEmpty())
			})

			It("the process runs in the container", func() {
				cmd := exec.Command(wincBin, append(commandArgs, "powershell.exe")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				pl := containerProcesses(containerId, "powershell.exe")
				Expect(len(pl)).To(Equal(1))

				state, err := cm.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(isParentOf(state.Pid, int(pl[0].ProcessId))).To(BeTrue())
			})

			Context("when the command is invalid", func() {
				It("errors", func() {
					cmd := exec.Command(wincBin, append(commandArgs, "invalid.exe")...)
					session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1))
					expectedError := &hcsclient.CouldNotCreateProcessError{Id: containerId, Command: "invalid.exe"}
					Expect(session.Err).To(gbytes.Say(expectedError.Error()))
				})
			})
		})
	})
})
