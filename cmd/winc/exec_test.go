package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Exec", func() {
	sendCtrlBreak := func(s *gexec.Session) {
		d, err := syscall.LoadDLL("kernel32.dll")
		Expect(err).ToNot(HaveOccurred())
		p, err := d.FindProc("GenerateConsoleCtrlEvent")
		Expect(err).ToNot(HaveOccurred())
		r, _, err := p.Call(syscall.CTRL_BREAK_EVENT, uintptr(s.Command.Process.Pid))
		Expect(r).ToNot(Equal(0), fmt.Sprintf("GenerateConsoleCtrlEvent: %v\n", err))
	}

	Context("when the container exists", func() {
		var (
			containerId string
		)

		BeforeEach(func() {
			containerId = filepath.Base(bundlePath)

			bundleSpec := runtimeSpecGenerator(createSandbox(rootPath, rootfsPath, containerId), containerId)
			config, err := json.Marshal(&bundleSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0666)).To(Succeed())
			_, _, err = execute(exec.Command(wincBin, "create", "-b", bundlePath, containerId))
			Expect(err).NotTo(HaveOccurred())

			pl := containerProcesses(containerId, "cmd.exe")
			Expect(pl).To(BeEmpty())
		})

		AfterEach(func() {
			_, _, err := execute(exec.Command(wincBin, "delete", containerId))
			Expect(err).NotTo(HaveOccurred())
			_, _, err = execute(exec.Command(wincImageBin, "--store", rootPath, "delete", containerId))
			Expect(err).NotTo(HaveOccurred())
		})

		It("the process runs in the container", func() {
			_, _, err := execute(exec.Command(wincBin, "exec", "--detach", containerId, "cmd.exe", "/C", "waitfor ever /T 9999"))
			Expect(err).ToNot(HaveOccurred())

			pl := containerProcesses(containerId, "cmd.exe")
			Expect(len(pl)).To(Equal(1))

			containerPid := getContainerState(containerId).Pid
			Expect(isParentOf(containerPid, int(pl[0].ProcessId))).To(BeTrue())
		})

		Context("when unix path is defined", func() {
			It("the process runs in the container", func() {
				_, _, err := execute(exec.Command(wincBin, "exec", "--detach", containerId, "/Windows/System32/cmd", "/C", "waitfor ever /T 9999"))
				Expect(err).ToNot(HaveOccurred())

				pl := containerProcesses(containerId, "cmd.exe")
				Expect(len(pl)).To(Equal(1))

				containerPid := getContainerState(containerId).Pid
				Expect(isParentOf(containerPid, int(pl[0].ProcessId))).To(BeTrue())
			})
		})

		Context("when there is cmd.exe and cmd", func() {
			BeforeEach(func() {
				containerPid := getContainerState(containerId).Pid
				cmdPath := filepath.Join("c:\\", "proc", strconv.Itoa(containerPid), "root", "Windows", "System32", "cmd")
				Expect(ioutil.WriteFile(cmdPath, []byte("xxx"), 0644)).To(Succeed())
			})

			It("runs the .exe for windows", func() {
				stdOut, _, err := execute(exec.Command(wincBin, "exec", containerId, "/Windows/System32/cmd", "/C", "echo app is running"))
				Expect(err).ToNot(HaveOccurred())
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
				expectedSpec.Args = []string{"cmd.exe", "/C", "waitfor ever /T 9999"}
				config, err := json.Marshal(&expectedSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(processConfig, config, 0666)).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(processConfig)).To(Succeed())
			})

			It("runs the process specified in the process.json", func() {
				_, _, err := execute(exec.Command(wincBin, "exec", "--process", processConfig, "--detach", containerId))
				Expect(err).NotTo(HaveOccurred())

				pl := containerProcesses(containerId, "cmd.exe")
				Expect(len(pl)).To(Equal(1))

				containerPid := getContainerState(containerId).Pid
				Expect(isParentOf(containerPid, int(pl[0].ProcessId))).To(BeTrue())
			})
		})

		Context("when the '--cwd' flag is provided", func() {
			It("runs the process in the specified directory", func() {
				stdOut, _, err := execute(exec.Command(wincBin, "exec", "--cwd", "C:\\Users", containerId, "cmd.exe", "/C", "echo %CD%"))
				Expect(err).NotTo(HaveOccurred())
				Expect(stdOut.String()).To(ContainSubstring("C:\\Users"))
			})
		})

		Context("when the '--user' flag is provided", func() {
			It("runs the process as the specified user", func() {
				stdOut, _, err := execute(exec.Command(wincBin, "exec", "--user", "vcap", containerId, "cmd.exe", "/C", "echo %USERNAME%"))
				Expect(err).NotTo(HaveOccurred())
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
					stdErr := new(bytes.Buffer)
					session, err := gexec.Start(cmd, GinkgoWriter, io.MultiWriter(stdErr, GinkgoWriter))
					Expect(err).ToNot(HaveOccurred())
					Eventually(session).Should(gexec.Exit(1))
					expectedErrorMsg := fmt.Sprintf("could not start command 'cmd.exe' in container: %s", containerId)
					Expect(stdErr.String()).To(ContainSubstring(expectedErrorMsg))

					log, err := ioutil.ReadFile(logFile)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(log)).To(ContainSubstring("The user name or password is incorrect."))
				})
			})
		})

		Context("when the '--env' flag is provided", func() {
			It("runs the process with the specified environment variables", func() {
				stdOut, _, err := execute(exec.Command(wincBin, "exec", "--env", "var1=foo", "--env", "var2=bar", containerId, "cmd.exe", "/C", "set"))
				Expect(err).ToNot(HaveOccurred())
				Expect(stdOut.String()).To(ContainSubstring("\nvar1=foo"))
				Expect(stdOut.String()).To(ContainSubstring("\nvar2=bar"))
			})
		})

		Context("when the --detach flag is passed", func() {
			It("the process runs in the container and returns immediately", func() {
				_, _, err := execute(exec.Command(wincBin, "exec", "--detach", containerId, "cmd.exe", "/C", "waitfor fivesec /T 5 >NULL & exit /B 0"))
				Expect(err).ToNot(HaveOccurred())

				pl := containerProcesses(containerId, "cmd.exe")
				Expect(len(pl)).To(Equal(1))

				containerPid := getContainerState(containerId).Pid
				Expect(isParentOf(containerPid, int(pl[0].ProcessId))).To(BeTrue())

				Eventually(func() []hcsshim.ProcessListItem {
					return containerProcesses(containerId, "cmd.exe")
				}, "10s").Should(BeEmpty())
			})
		})

		Context("when the --detach flag is not passed", func() {
			It("the process runs in the container and returns the exit code when the process finishes", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "exit /B 5")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(5))

				pl := containerProcesses(containerId, "cmd.exe")
				Expect(len(pl)).To(Equal(0))
			})

			It("passes stdin through to the process", func() {
				containerPid := getContainerState(containerId).Pid
				err := copy(filepath.Join("c:\\", "proc", strconv.Itoa(containerPid), "root", "read.exe"), readBin)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(wincBin, "exec", containerId, "c:\\read.exe")
				cmd.Stdin = strings.NewReader("hey-winc\n")
				stdOut, _, err := execute(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(stdOut.String()).To(ContainSubstring("hey-winc"))
			})

			It("captures the stdout", func() {
				stdOut, _, err := execute(exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "echo hey-winc"))
				Expect(err).NotTo(HaveOccurred())
				Expect(stdOut.String()).To(ContainSubstring("hey-winc"))
			})

			It("captures the stderr", func() {
				_, stdErr, err := execute(exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "echo hey-winc 1>&2"))
				Expect(err).ToNot(HaveOccurred())
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
				pl := containerProcesses(containerId, "cmd.exe")
				Expect(len(pl)).To(Equal(1))

				sendCtrlBreak(session)
				Eventually(session).Should(gexec.Exit(1067))
				pl = containerProcesses(containerId, "cmd.exe")
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
				_, _, err := execute(exec.Command(wincBin, "exec", "--detach", "--pid-file", pidFile, containerId, "cmd.exe", "/C", "waitfor ever /T 9999"))
				Expect(err).ToNot(HaveOccurred())

				pl := containerProcesses(containerId, "cmd.exe")
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
				stdErr := new(bytes.Buffer)
				session, err := gexec.Start(cmd, GinkgoWriter, io.MultiWriter(stdErr, GinkgoWriter))
				Expect(err).ToNot(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				expectedErrorMsg := fmt.Sprintf("could not start command 'invalid.exe' in container: %s", containerId)
				Expect(stdErr.String()).To(ContainSubstring(expectedErrorMsg))
			})
		})
	})

	Context("given a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "exec", "doesntexist", "cmd.exe")
			stdErr := new(bytes.Buffer)
			session, err := gexec.Start(cmd, GinkgoWriter, io.MultiWriter(stdErr, GinkgoWriter))
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(stdErr.String()).To(ContainSubstring("container not found: doesntexist"))
		})
	})
})
