package main_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

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

			Context("when the --detach flag is passed", func() {
				It("the process runs in the container and returns immediately", func() {
					cmd := exec.Command(wincBin, "exec", "--detach", containerId, "powershell.exe", "-Command", "Start-Sleep -s 10")
					session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())
					Eventually(session).Should(gexec.Exit(0))

					pl := containerProcesses(containerId, "powershell.exe")
					Expect(len(pl)).To(Equal(1))

					state, err := cm.State()
					Expect(err).ToNot(HaveOccurred())
					Expect(isParentOf(state.Pid, int(pl[0].ProcessId))).To(BeTrue())
				})
			})

			Context("when the --detach flag is not passed", func() {
				It("the process runs in the container and returns the exit code when the process finishes", func() {
					cmd := exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", "Exit 5")
					session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())
					Eventually(session, "5s").Should(gexec.Exit(5))

					pl := containerProcesses(containerId, "powershell.exe")
					Expect(len(pl)).To(Equal(0))
				})
			})

			Context("when the '--pid-file' flag is provided", func() {
				var pidFile string

				BeforeEach(func() {
					pidFile = filepath.Join(os.TempDir(), "pidfile")
				})

				AfterEach(func() {
					Expect(os.RemoveAll(pidFile)).To(Succeed())
				})

				It("places the started process id in the specified file", func() {
					cmd := exec.Command(wincBin, "exec", "--detach", "--pid-file", pidFile, containerId, "powershell.exe")
					session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())
					Eventually(session).Should(gexec.Exit(0))

					pl := containerProcesses(containerId, "powershell.exe")
					Expect(len(pl)).To(Equal(1))

					pidBytes, err := ioutil.ReadFile(pidFile)
					Expect(err).ToNot(HaveOccurred())
					pid, err := strconv.ParseInt(string(pidBytes), 10, 64)
					Expect(err).ToNot(HaveOccurred())
					Expect(int(pid)).To(Equal(int(pl[0].ProcessId)))
				})
			})

			Context("when the command is invalid", func() {
				It("errors", func() {
					cmd := exec.Command(wincBin, "exec", containerId, "invalid.exe")
					session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())

					Eventually(session).Should(gexec.Exit(1))
					expectedError := &hcsclient.CouldNotCreateProcessError{Id: containerId, Command: "invalid.exe"}
					Expect(session.Err).To(gbytes.Say(expectedError.Error()))
				})
			})
		})
	})

	Context("given a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "exec", "doesntexist", "powershell.exe")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &hcsclient.NotFoundError{Id: "doesntexist"}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))
		})
	})
})
