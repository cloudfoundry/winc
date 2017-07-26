package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/mounter"
	"code.cloudfoundry.org/winc/sandbox"
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
		stdOut      *bytes.Buffer
		stdErr      *bytes.Buffer
	)

	sendCtrlBreak := func(s *gexec.Session) {
		d, err := syscall.LoadDLL("kernel32.dll")
		Expect(err).ToNot(HaveOccurred())
		p, err := d.FindProc("GenerateConsoleCtrlEvent")
		Expect(err).ToNot(HaveOccurred())
		r, _, err := p.Call(syscall.CTRL_BREAK_EVENT, uintptr(s.Command.Process.Pid))
		Expect(r).ToNot(Equal(0), fmt.Sprintf("GenerateConsoleCtrlEvent: %v\n", err))
	}

	BeforeEach(func() {
		containerId = strconv.Itoa(rand.Int())
		bundlePath = filepath.Join(depotDir, containerId)

		Expect(os.MkdirAll(bundlePath, 0755)).To(Succeed())

		client = hcsclient.HCSClient{}
		sm := sandbox.NewManager(&client, &mounter.Mounter{}, depotDir, containerId)
		nm := networkManager(&client)
		cm = container.NewManager(&client, sm, nm, bundlePath)

		stdOut = new(bytes.Buffer)
		stdErr = new(bytes.Buffer)
	})

	Context("when the container exists", func() {
		BeforeEach(func() {
			bundleSpec := runtimeSpecGenerator(rootfsPath)
			Expect(cm.Create(&bundleSpec)).To(Succeed())
			pl := containerProcesses(&client, containerId, "cmd.exe")
			Expect(pl).To(BeEmpty())
		})

		AfterEach(func() {
			Expect(cm.Delete()).To(Succeed())
		})

		It("the process runs in the container", func() {
			err := exec.Command(wincBin, "exec", "--detach", containerId, "cmd.exe").Run()
			Expect(err).ToNot(HaveOccurred())

			pl := containerProcesses(&client, containerId, "cmd.exe")
			Expect(len(pl)).To(Equal(1))

			state, err := cm.State()
			Expect(err).ToNot(HaveOccurred())
			Expect(isParentOf(state.Pid, int(pl[0].ProcessId))).To(BeTrue())
		})

		Context("when unix path is defined", func() {
			It("the process runs in the container", func() {
				err := exec.Command(wincBin, "exec", "--detach", containerId, "/Windows/System32/cmd").Run()
				Expect(err).ToNot(HaveOccurred())

				pl := containerProcesses(&client, containerId, "cmd.exe")
				Expect(len(pl)).To(Equal(1))

				state, err := cm.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(isParentOf(state.Pid, int(pl[0].ProcessId))).To(BeTrue())
			})
		})

		Context("when there is cmd.exe and cmd", func() {
			BeforeEach(func() {
				state, err := cm.State()
				Expect(err).To(Succeed())
				cmdPath := filepath.Join("c:\\", "proc", strconv.Itoa(state.Pid), "root", "Windows", "System32", "cmd")
				Expect(ioutil.WriteFile(cmdPath, []byte("xxx"), 0644)).To(Succeed())
			})

			It("runs the .exe for windows", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "/Windows/System32/cmd", "/C", "echo app is running")
				cmd.Stdout = stdOut
				Expect(cmd.Run()).To(Succeed())
				Expect(stdOut.String()).To(ContainSubstring("app is running"))
			})
		})

		Context("when the '--process' flag is provided", func() {
			var processConfig string

			BeforeEach(func() {
				f, err := ioutil.TempFile("", "process.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(f.Close()).To(Succeed())
				processConfig = f.Name()
				expectedSpec := processSpecGenerator()
				config, err := json.Marshal(&expectedSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(processConfig, config, 0666)).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(processConfig)).To(Succeed())
			})

			It("runs the process specified in the process.json", func() {
				err := exec.Command(wincBin, "exec", "--process", processConfig, "--detach", containerId).Run()
				Expect(err).ToNot(HaveOccurred())

				pl := containerProcesses(&client, containerId, "cmd.exe")
				Expect(len(pl)).To(Equal(1))

				state, err := cm.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(isParentOf(state.Pid, int(pl[0].ProcessId))).To(BeTrue())
			})
		})

		Context("when the '--cwd' flag is provided", func() {
			It("runs the process in the specified directory", func() {
				cmd := exec.Command(wincBin, "exec", "--cwd", "C:\\Users", containerId, "cmd.exe", "/C", "echo %CD%")
				cmd.Stdout = stdOut
				Expect(cmd.Run()).To(Succeed())
				Expect(stdOut.String()).To(ContainSubstring("C:\\Users"))
			})
		})

		Context("when the '--user' flag is provided", func() {
			It("runs the process as the specified user", func() {
				cmd := exec.Command(wincBin, "exec", "--user", "vcap", containerId, "cmd.exe", "/C", "echo %USERNAME%")
				cmd.Stdout = stdOut
				Expect(cmd.Run()).To(Succeed())
				Expect(stdOut.String()).To(ContainSubstring("vcap"))
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
					cmd := exec.Command(wincBin, "--log", logFile, "--debug", "exec", "--user", "doesntexist", containerId, "cmd.exe", "/C", "echo %USERNAME%")
					session, err := gexec.Start(cmd, stdOut, stdErr)
					Expect(err).ToNot(HaveOccurred())
					Eventually(session).Should(gexec.Exit(1))
					expectedError := &hcsclient.CouldNotCreateProcessError{Id: containerId, Command: "cmd.exe"}
					Expect(stdErr.String()).To(ContainSubstring(expectedError.Error()))

					log, err := ioutil.ReadFile(logFile)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(log)).To(ContainSubstring("The user name or password is incorrect."))
				})
			})
		})

		Context("when the '--env' flag is provided", func() {
			It("runs the process with the specified environment variables", func() {
				cmd := exec.Command(wincBin, "exec", "--env", "var1=foo", "--env", "var2=bar", containerId, "cmd.exe", "/C", "set")
				cmd.Stdout = stdOut
				Expect(cmd.Run()).To(Succeed())
				Expect(stdOut.String()).To(ContainSubstring("\nvar1=foo"))
				Expect(stdOut.String()).To(ContainSubstring("\nvar2=bar"))
			})
		})

		Context("when the --detach flag is passed", func() {
			It("the process runs in the container and returns immediately", func() {
				err := exec.Command(wincBin, "exec", "--detach", containerId, "cmd.exe", "/C", "waitfor tensec /T 10 >NULL & exit /B 0").Run()
				Expect(err).ToNot(HaveOccurred())

				pl := containerProcesses(&client, containerId, "cmd.exe")
				Expect(len(pl)).To(Equal(1))

				state, err := cm.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(isParentOf(state.Pid, int(pl[0].ProcessId))).To(BeTrue())
			})
		})

		Context("when the --detach flag is not passed", func() {
			It("the process runs in the container and returns the exit code when the process finishes", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "exit /B 5")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(5))

				pl := containerProcesses(&client, containerId, "cmd.exe")
				Expect(len(pl)).To(Equal(0))
			})

			It("passes stdin through to the process", func() {
				state, err := cm.State()
				Expect(err).ToNot(HaveOccurred())

				err = copy(filepath.Join("c:\\", "proc", strconv.Itoa(state.Pid), "root", "read.exe"), readBin)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(wincBin, "exec", containerId, "c:\\read.exe")
				cmd.Stdin = strings.NewReader("hey-winc\n")
				cmd.Stdout = stdOut
				Expect(cmd.Run()).To(Succeed())
				Expect(stdOut.String()).To(ContainSubstring("hey-winc"))
			})

			It("captures the stdout", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "echo hey-winc")
				cmd.Stdout = stdOut
				Expect(cmd.Run()).To(Succeed())
				Expect(stdOut.String()).To(ContainSubstring("hey-winc"))
			})

			It("captures the stderr", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "echo hey-winc 1>&2 && exit /B 5")
				session, err := gexec.Start(cmd, stdOut, stdErr)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(5))
				Expect(stdErr.String()).To(ContainSubstring("hey-winc"))
			})

			It("captures the CTRL+C", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "echo hey-winc & waitfor ever /T 9999")
				cmd.SysProcAttr = &syscall.SysProcAttr{
					CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
				}
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Consistently(session).ShouldNot(gexec.Exit(0))
				Eventually(session.Out).Should(gbytes.Say("hey-winc"))
				pl := containerProcesses(&client, containerId, "cmd.exe")
				Expect(len(pl)).To(Equal(1))

				sendCtrlBreak(session)
				Eventually(session).Should(gexec.Exit(1067))
				pl = containerProcesses(&client, containerId, "cmd.exe")
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
				err := exec.Command(wincBin, "exec", "--detach", "--pid-file", pidFile, containerId, "cmd.exe").Run()
				Expect(err).ToNot(HaveOccurred())

				pl := containerProcesses(&client, containerId, "cmd.exe")
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
				session, err := gexec.Start(cmd, stdOut, stdErr)
				Expect(err).ToNot(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				expectedError := &hcsclient.CouldNotCreateProcessError{Id: containerId, Command: "invalid.exe"}
				Expect(stdErr.String()).To(ContainSubstring(expectedError.Error()))
			})
		})
	})

	Context("given a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "exec", "doesntexist", "cmd.exe")
			session, err := gexec.Start(cmd, stdOut, stdErr)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &hcsclient.NotFoundError{Id: "doesntexist"}
			Expect(stdErr.String()).To(ContainSubstring(expectedError.Error()))
		})
	})
})
