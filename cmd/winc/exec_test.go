package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

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

	sendCtrlBreak := func(s *gexec.Session) {
		d, err := syscall.LoadDLL("kernel32.dll")
		Expect(err).ToNot(HaveOccurred())
		p, err := d.FindProc("GenerateConsoleCtrlEvent")
		Expect(err).ToNot(HaveOccurred())
		r, _, err := p.Call(syscall.CTRL_BREAK_EVENT, uintptr(s.Command.Process.Pid))
		Expect(r).ToNot(Equal(0), fmt.Sprintf("GenerateConsoleCtrlEvent: %v\n", err))
	}

	BeforeEach(func() {
		containerId = filepath.Base(bundlePath)

		client = hcsclient.HCSClient{}
		sm := sandbox.NewManager(&client, bundlePath)
		cm = container.NewManager(&client, sm, containerId)
	})

	Context("when the container exists", func() {
		BeforeEach(func() {
			bundleSpec := runtimeSpecGenerator(rootfsPath)
			Expect(cm.Create(&bundleSpec)).To(Succeed())
			pl := containerProcesses(containerId, "powershell.exe")
			Expect(pl).To(BeEmpty())
		})

		AfterEach(func() {
			Expect(cm.Delete()).To(Succeed())
		})

		It("the process runs in the container", func() {
			cmd := exec.Command(wincBin, "exec", "--detach", containerId, "powershell.exe")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			pl := containerProcesses(containerId, "powershell.exe")
			Expect(len(pl)).To(Equal(1))

			state, err := cm.State()
			Expect(err).ToNot(HaveOccurred())
			Expect(isParentOf(state.Pid, int(pl[0].ProcessId))).To(BeTrue())
		})

		Context("when the '--process' flag is provided", func() {
			var processConfig string

			BeforeEach(func() {
				f, err := ioutil.TempFile("", "process.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(f.Close()).To(Succeed())
				processConfig = f.Name()
				expectedSpec := processSpecGenerator()
				expectedSpec.User.Username = "Guest"
				config, err := json.Marshal(&expectedSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(processConfig, config, 0666)).To(Succeed())

				cmd := exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", "Enable-LocalUser -Name Guest")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
			})

			AfterEach(func() {
				Expect(os.RemoveAll(processConfig)).To(Succeed())
			})

			It("runs the process specified in the process.json", func() {
				cmd := exec.Command(wincBin, "exec", "--process", processConfig, "--detach", containerId)
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

		Context("when the '--cwd' flag is provided", func() {
			It("runs the process in the specified directory", func() {
				cmd := exec.Command(wincBin, "exec", "--cwd", "C:\\Users", containerId, "powershell.exe", "-Command", "Write-Host $PWD")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say(`C:\\Users`))
			})
		})

		Context("when the '--user' flag is provided", func() {
			BeforeEach(func() {
				cmd := exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", "Enable-LocalUser -Name Guest")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
			})

			It("runs the process as the specified user", func() {
				cmd := exec.Command(wincBin, "exec", "--user", "Guest", containerId, "powershell.exe", "-Command", "Write-Host $env:UserName")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say("Guest"))
			})

			Context("when the specified user does not exist or cannot be used", func() {
				var logFile string

				BeforeEach(func() {
					f, err := ioutil.TempFile("", "winc.log")
					Expect(err).ToNot(HaveOccurred())
					Expect(f.Close()).To(Succeed())
					logFile = f.Name()
				})

				AfterEach(func() {
					Expect(os.RemoveAll(logFile)).To(Succeed())
				})

				It("errors", func() {
					cmd := exec.Command(wincBin, "--log", logFile, "--debug", "exec", "--user", "doesntexist", containerId, "powershell.exe", "-Command", "Write-Host $env:UserName")
					session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())
					Eventually(session).Should(gexec.Exit(1))
					expectedError := &hcsclient.CouldNotCreateProcessError{Id: containerId, Command: "powershell.exe"}
					Expect(session.Err).To(gbytes.Say(expectedError.Error()))

					log, err := ioutil.ReadFile(logFile)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(log)).To(ContainSubstring("The user name or password is incorrect."))
				})
			})
		})

		Context("when the '--env' flag is provided", func() {
			It("runs the process with the specified environment variables", func() {
				cmd := exec.Command(wincBin, "exec", "--env", "var1=foo", "--env", "var2=bar", containerId, "powershell.exe", "-Command", "Get-ChildItem Env:")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say(`\nvar1\s+foo`))
				Expect(session.Out).To(gbytes.Say(`\nvar2\s+bar`))
			})
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
				Eventually(session).Should(gexec.Exit(5))

				pl := containerProcesses(containerId, "powershell.exe")
				Expect(len(pl)).To(Equal(0))
			})

			It("captures the stdout", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", "Write-Host hey-winc")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session, "10s").Should(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say("hey-winc"))
			})

			It("captures the stderr", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", "Write-Error hey-winc; Exit 5;")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session, "10s").Should(gexec.Exit(5))
				Expect(session.Err).To(gbytes.Say("hey-winc"))
			})

			It("captures the CTRL+C", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", "While($true) {Write-Host hey-winc; Start-Sleep 1;}")
				cmd.SysProcAttr = &syscall.SysProcAttr{
					CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
				}
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Consistently(session).ShouldNot(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say("hey-winc"))
				pl := containerProcesses(containerId, "powershell.exe")
				Expect(len(pl)).To(Equal(1))

				sendCtrlBreak(session)
				Eventually(session).Should(gexec.Exit(1067))
				pl = containerProcesses(containerId, "powershell.exe")
				Expect(len(pl)).To(Equal(0))
			})
		})

		Context("when the '--pid-file' flag is provided", func() {
			var pidFile string

			BeforeEach(func() {
				f, err := ioutil.TempFile("", "pid")
				Expect(err).ToNot(HaveOccurred())
				Expect(f.Close()).To(Succeed())
				pidFile = f.Name()
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
