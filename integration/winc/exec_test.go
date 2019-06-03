package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Microsoft/hcsshim"
	acl "github.com/hectane/go-acl"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/windows"
)

var _ = Describe("Exec", func() {
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

			bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
			bundleSpec.Mounts = []specs.Mount{{Source: filepath.Dir(sleepBin), Destination: "C:\\tmp"}}
			Expect(acl.Apply(filepath.Dir(sleepBin), false, false, acl.GrantName(windows.GENERIC_ALL, "Everyone"))).To(Succeed())
			helpers.RunContainer(bundleSpec, bundlePath, containerId)
		})

		AfterEach(func() {
			failed = failed || CurrentGinkgoTestDescription().Failed
			helpers.DeleteContainer(containerId)
			helpers.DeleteVolume(containerId)
			Expect(os.RemoveAll(bundlePath)).To(Succeed())
		})

		It("the process runs in the container", func() {
			stdOut, stdErr, err := helpers.ExecInContainer(containerId, []string{"C:\\tmp\\sleep.exe"}, true)
			Expect(err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())

			pl := helpers.ContainerProcesses(containerId, "sleep.exe")
			Expect(len(pl)).To(Equal(1))
		})

		It("runs an executible given a unix path in the container", func() {
			stdOut, stdErr, err := helpers.ExecInContainer(containerId, []string{"/tmp/sleep"}, true)
			Expect(err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())

			pl := helpers.ContainerProcesses(containerId, "sleep.exe")
			Expect(len(pl)).To(Equal(1))
		})

		Context("when there is cmd.exe and cmd", func() {
			BeforeEach(func() {
				containerPid := helpers.GetContainerState(containerId).Pid
				cmdPath := filepath.Join("c:\\", "proc", strconv.Itoa(containerPid), "root", "Windows", "System32", "cmd")
				Expect(ioutil.WriteFile(cmdPath, []byte("xxx"), 0644)).To(Succeed())
			})

			It("runs the .exe for windows", func() {
				stdOut, stdErr, err := helpers.ExecInContainer(containerId, []string{"/Windows/System32/cmd", "/C", "echo app is running"}, false)
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
			})

			AfterEach(func() {
				Expect(os.RemoveAll(processConfig)).To(Succeed())
			})

			It("runs the process specified in the process.json", func() {
				expectedSpec := processSpecGenerator()
				expectedSpec.Args = []string{"/tmp/sleep", "99999"}
				config, err := json.Marshal(&expectedSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(processConfig, config, 0666)).To(Succeed())

				args := []string{"exec", "--process", processConfig, "--detach", containerId}
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				pl := helpers.ContainerProcesses(containerId, "sleep.exe")
				Expect(len(pl)).To(Equal(1))
			})

			It("cleans errors returned from hcsshim", func() {
				expectedSpec := processSpecGenerator()
				expectedSpec.Args = []string{"some-invalid-command"}
				config, err := json.Marshal(&expectedSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(processConfig, config, 0666)).To(Succeed())

				args := []string{"exec", "--process", processConfig, containerId}
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())
				Expect(stdOut.String()).To(BeEmpty())
				Expect(strings.TrimSpace(stdErr.String())).To(Equal(fmt.Sprintf("The system cannot find the file specified.: could not start command 'some-invalid-command.exe' in container: %s", containerId)))
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
			BeforeEach(func() {
				args := []string{"exec", containerId, "cmd.exe", "/C", "net user alice /ADD /passwordreq:no && runas /user:alice whoami"}
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
			})

			It("runs the process as the specified user", func() {
				args := []string{"--debug", "exec", "--user", "alice", containerId, "cmd.exe", "/C", "echo %USERNAME%"}
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, args...))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				Expect(stdOut.String()).To(ContainSubstring("alice"))
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
				stdOut, stdErr, err := helpers.ExecInContainer(containerId, []string{"/tmp/sleep", "5"}, true)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				pl := helpers.ContainerProcesses(containerId, "sleep.exe")
				Expect(len(pl)).To(Equal(1))

				Eventually(func() []hcsshim.ProcessListItem {
					return helpers.ContainerProcesses(containerId, "sleep.exe")
				}, "10s").Should(BeEmpty())
			})
		})

		Context("when the --detach flag is not passed", func() {
			It("the process runs in the container and returns the exit code when the process finishes", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "exit /B 5")
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(5))

				pl := helpers.ContainerProcesses(containerId, "cmd.exe")
				Expect(len(pl)).To(Equal(0))
			})

			It("passes stdin through to the process", func() {
				cmd := exec.Command(wincBin, "exec", containerId, "findstr", ".*")
				cmd.Stdin = strings.NewReader("hey-winc")
				stdOut, stdErr, err := helpers.Execute(cmd)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
				Expect(stdOut.String()).To(ContainSubstring("hey-winc"))
			})

			It("captures the stdout", func() {
				stdOut, stdErr, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "echo hey-winc"}, false)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
				Expect(stdOut.String()).To(ContainSubstring("hey-winc"))
			})

			It("captures the stderr", func() {
				stdOut, stdErr, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "echo hey-winc 1>&2"}, false)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
				Expect(stdErr.String()).To(ContainSubstring("hey-winc"))
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

				pl := helpers.ContainerProcesses(containerId, "cmd.exe")
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
				stdOut, stdErr, err := helpers.ExecInContainer(containerId, []string{"invalid.exe"}, false)
				Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())

				expectedErrorMsg := fmt.Sprintf("could not start command 'invalid.exe' in container: %s", containerId)
				Expect(stdErr.String()).To(ContainSubstring(expectedErrorMsg))
			})
		})
	})

	Context("given a nonexistent container id", func() {
		It("errors", func() {
			stdOut, stdErr, err := helpers.ExecInContainer("doesntexist", []string{"cmd.exe"}, false)
			Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())

			Expect(stdErr.String()).To(ContainSubstring("hcsshim::OpenComputeSystem doesntexist"))
			Expect(stdErr.String()).To(ContainSubstring("the specified identifier does not exist"))
		})
	})

	Context("handles graceful shutdown when 2019 cf rootfs is used", func() {
		var (
			containerId string
			bundlePath  string
			bundleSpec  specs.Spec
		)

		BeforeEach(func() {
			var err error
			if !strings.HasPrefix(os.Getenv("WINC_TEST_ROOTFS"), "docker:///cloudfoundry/windows2016fs:2019") {
				Skip("This test is relevant only for cloudfoundry/windows2016fs:2019* rootfs")
			}
			bundlePath, err = ioutil.TempDir("", "winccontainer")
			Expect(err).To(Succeed())

			containerId = filepath.Base(bundlePath)

			bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
			bundleSpec.Mounts = []specs.Mount{{Source: filepath.Dir(goshutBin), Destination: "C:\\tmp"}}
			Expect(acl.Apply(filepath.Dir(goshutBin), false, false, acl.GrantName(windows.GENERIC_ALL, "Everyone"))).To(Succeed())
			helpers.RunContainer(bundleSpec, bundlePath, containerId)
		})

		AfterEach(func() {
			failed = failed || CurrentGinkgoTestDescription().Failed
			helpers.DeleteVolume(containerId)
			Expect(os.RemoveAll(bundlePath)).To(Succeed())
		})

		Context("when the container process has a CTRL_SHUTDOWN_EVENT handler that prints output forever", func() {
			It("should run for 10s after winc delete", func() {
				// goshut.exe will print a line forever roughly every 500ms once it receives CTRL_SHUTDOWN_EVENT
				cmd := exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "C:\\tmp\\goshut.exe")
				wincExecSession, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(wincExecSession.Out).Should(gbytes.Say("Starting goshut"))

				cmd2 := exec.Command(wincBin, "delete", containerId)
				_, err = gexec.Start(cmd2, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				// Assert something is logged from the 9th second of the container process.
				// e.g. IN LOOP: Elapsed time=9.5301979s
				Eventually(wincExecSession.Out).Should(gbytes.Say("IN LOOP: Elapsed time=9"))
				Eventually(wincExecSession.Out).ShouldNot(gbytes.Say("IN LOOP: Elapsed time=12"))
			})
		})

		Context("when the container process does NOT have a CTRL_SHUTDOWN_EVENT handler", func() {
			It("should exit without waiting for graceful shutdown time, on winc delete", func() {
				cmdstr := fmt.Sprintf("echo hey-winc; %s exec %s powershell start-sleep 100000; Get-Date -UFormat %%s", wincBin, containerId)
				cmd := exec.Command("powershell", "-c", cmdstr)
				wincExecSession, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(wincExecSession.Out).Should(gbytes.Say("hey-winc"))

				cmdstr2 := fmt.Sprintf("Get-Date -UFormat %%s; %s delete %s", wincBin, containerId)
				cmd2 := exec.Command("powershell", "-c", cmdstr2)
				wincDeleteSession, err := gexec.Start(cmd2, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				/*
				* You'd think why didn't these folks use ginkgo's regular `session.Wait()`
				* and measure the time difference between a `winc delete` and when `winc exec`
				* exits. But this does not give us the kind of precision (time) required
				* in this test
				 */
				Eventually(func() float64 {
					floatRegex := regexp.MustCompile(`\d+\.\d+`)

					wincDeleteTime := floatRegex.Find(wincDeleteSession.Out.Contents())
					t1, _ := strconv.ParseFloat(string(wincDeleteTime), 64)

					wincExecTime := floatRegex.Find(wincExecSession.Out.Contents())
					t2, _ := strconv.ParseFloat(string(wincExecTime), 64)

					return t2 - t1
				}).Should(SatisfyAll(
					BeNumerically(">", 0.0),
					BeNumerically("<", 1.0)))
			})
		})
	})
})
