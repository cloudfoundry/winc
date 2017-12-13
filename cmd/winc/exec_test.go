package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	helpers "code.cloudfoundry.org/winc/cmd/helpers"
	"github.com/Microsoft/hcsshim"
	acl "github.com/hectane/go-acl"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/windows"
)

var _ = FDescribe("Exec", func() {
	Context("when the container exists", func() {
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

			bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateSandbox(wincImageBin, imageStore, rootfsPath, containerId))
			bundleSpec.Mounts = []specs.Mount{{Source: filepath.Dir(sleepBin), Destination: "C:\\tmp"}}
			Expect(acl.Apply(filepath.Dir(sleepBin), false, false, acl.GrantName(windows.GENERIC_ALL, "Everyone"))).To(Succeed())
			wincBinGenericCreate(bundleSpec, bundlePath, containerId)
		})

		AfterEach(func() {
			helpers.DeleteContainer(wincBin, containerId)
			helpers.DeleteSandbox(wincImageBin, imageStore, containerId)
			Expect(os.RemoveAll(bundlePath)).To(Succeed())
		})

		It("the process runs in the container", func() {
			stdOut, stdErr, err := helpers.ExecInContainer(wincBin, containerId, []string{"C:\\tmp\\sleep.exe"}, true)
			Expect(err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())

			pl := containerProcesses(containerId, "sleep.exe")
			Expect(len(pl)).To(Equal(1))

			containerPid := helpers.GetContainerState(wincBin, containerId).Pid
			Expect(isParentOf(containerPid, int(pl[0].ProcessId))).To(BeTrue())
		})

		It("runs an executible given a unix path in the container", func() {
			stdOut, stdErr, err := helpers.ExecInContainer(wincBin, containerId, []string{"/tmp/sleep"}, true)
			Expect(err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())

			pl := containerProcesses(containerId, "sleep.exe")
			Expect(len(pl)).To(Equal(1))

			containerPid := helpers.GetContainerState(wincBin, containerId).Pid
			Expect(isParentOf(containerPid, int(pl[0].ProcessId))).To(BeTrue())
		})

		Context("when there is cmd.exe and cmd", func() {
			BeforeEach(func() {
				containerPid := helpers.GetContainerState(wincBin, containerId).Pid
				cmdPath := filepath.Join("c:\\", "proc", strconv.Itoa(containerPid), "root", "Windows", "System32", "cmd")
				Expect(ioutil.WriteFile(cmdPath, []byte("xxx"), 0644)).To(Succeed())
			})

			It("runs the .exe for windows", func() {
				stdOut, stdErr, err := helpers.ExecInContainer(wincBin, containerId, []string{"/Windows/System32/cmd", "/C", "echo app is running"}, false)
				Expect(err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())
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
				expectedSpec.Args = []string{"/tmp/sleep", "99999"}
				config, err := json.Marshal(&expectedSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(processConfig, config, 0666)).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(processConfig)).To(Succeed())
			})

			It("runs the process specified in the process.json", func() {
				args := []string{"exec", "--process", processConfig, "--detach", containerId}
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				pl := containerProcesses(containerId, "sleep.exe")
				Expect(len(pl)).To(Equal(1))

				containerPid := helpers.GetContainerState(wincBin, containerId).Pid
				Expect(isParentOf(containerPid, int(pl[0].ProcessId))).To(BeTrue())
			})
		})

		Context("when the '--cwd' flag is provided", func() {
			It("runs the process in the specified directory", func() {
				args := []string{"exec", "--cwd", "C:\\Users", containerId, "cmd.exe", "/C", "echo %CD%"}
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				Expect(stdOut.String()).To(ContainSubstring("C:\\Users"))
			})
		})

		Context("when the '--user' flag is provided", func() {
			It("runs the process as the specified user", func() {
				stdOut, stdErr, err := helpers.ExecInContainer(wincBin, containerId, []string{"cmd.exe", "/C", "echo %USERNAME%"}, false)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

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
					args := []string{"--log", logFile, "--debug", "exec", "--user", "doesntexist", containerId, "cmd.exe", "/C", "echo %USERNAME%"}
					stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
					Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())

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
				args := []string{"exec", "--env", "var1=foo", "--env", "var2=bar", containerId, "cmd.exe", "/C", "set"}
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
				Expect(stdOut.String()).To(ContainSubstring("\nvar1=foo"))
				Expect(stdOut.String()).To(ContainSubstring("\nvar2=bar"))
			})
		})

		Context("when the --detach flag is passed", func() {
			It("the process runs in the container and returns immediately", func() {
				stdOut, stdErr, err := helpers.ExecInContainer(wincBin, containerId, []string{"/tmp/sleep", "5"}, true)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				pl := containerProcesses(containerId, "sleep.exe")
				Expect(len(pl)).To(Equal(1))

				containerPid := helpers.GetContainerState(wincBin, containerId).Pid
				Expect(isParentOf(containerPid, int(pl[0].ProcessId))).To(BeTrue())

				Eventually(func() []hcsshim.ProcessListItem {
					return containerProcesses(containerId, "sleep.exe")
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
				containerPid := helpers.GetContainerState(wincBin, containerId).Pid
				err := helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(containerPid), "root", "read.exe"), readBin)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(wincBin, "exec", containerId, "c:\\read.exe")
				cmd.Stdin = strings.NewReader("hey-winc\n")
				stdOut, stdErr, err := helpers.Execute(cmd)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
				Expect(stdOut.String()).To(ContainSubstring("hey-winc"))
			})

			It("captures the stdout", func() {
				stdOut, stdErr, err := helpers.ExecInContainer(wincBin, containerId, []string{"cmd.exe", "/C", "echo hey-winc"}, false)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
				Expect(stdOut.String()).To(ContainSubstring("hey-winc"))
			})

			It("captures the stderr", func() {
				stdOut, stdErr, err := helpers.ExecInContainer(wincBin, containerId, []string{"cmd.exe", "/C", "echo hey-winc 1>&2"}, false)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
				Expect(stdErr.String()).To(ContainSubstring("hey-winc"))
			})

			It("captures the CTRL+C", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "echo hey-winc & C:\\tmp\\sleep.exe 9999")
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
				args := []string{"exec", "--detach", "--pid-file", pidFile, containerId, "cmd.exe", "/C", "C:\\tmp\\sleep"}
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())

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
				stdOut, stdErr, err := helpers.ExecInContainer(wincBin, containerId, []string{"invalid.exe"}, false)
				Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())

				expectedErrorMsg := fmt.Sprintf("could not start command 'invalid.exe' in container: %s", containerId)
				Expect(stdErr.String()).To(ContainSubstring(expectedErrorMsg))
			})
		})
	})

	Context("given a nonexistent container id", func() {
		It("errors", func() {
			stdOut, stdErr, err := helpers.ExecInContainer(wincBin, "doesntexist", []string{"cmd.exe"}, false)
			Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())

			Expect(stdErr.String()).To(ContainSubstring("container not found: doesntexist"))
		})
	})
})
