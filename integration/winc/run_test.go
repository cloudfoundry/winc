package main_test

import (
	"bytes"
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
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = FDescribe("Run", func() {
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
	})

	AfterEach(func() {
		failed = failed || CurrentGinkgoTestDescription().Failed
		helpers.DeleteContainer(containerId)
		helpers.DeleteVolume(containerId)
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	It("creates a container and runs the init process", func() {
		helpers.GenerateBundle(bundleSpec, bundlePath)
		_, _, err := helpers.ExecuteNoOutput(exec.Command(wincBin, "run", "-b", bundlePath, "--detach", containerId))
		Expect(err).ToNot(HaveOccurred())

		Expect(helpers.ContainerExists(containerId)).To(BeTrue())

		pl := helpers.ContainerProcesses(containerId, "waitfor.exe")
		Expect(len(pl)).To(Equal(1))

		containerPid := helpers.GetContainerState(containerId).Pid
		Expect(pl[0].ProcessId).To(Equal(uint32(containerPid)))
	})

	It("mounts the sandbox.vhdx at C:\\proc\\<pid>\\root", func() {
		helpers.GenerateBundle(bundleSpec, bundlePath)
		_, _, err := helpers.ExecuteNoOutput(exec.Command(wincBin, "run", "-b", bundlePath, "--detach", containerId))
		Expect(err).ToNot(HaveOccurred())

		pid := helpers.GetContainerState(containerId).Pid
		Expect(ioutil.WriteFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "test.txt"), []byte("contents"), 0644)).To(Succeed())

		stdOut, _, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "type", "test.txt"}, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(stdOut.String()).To(ContainSubstring("contents"))
	})

	Context("when the bundle config.json specifies a container memory limit", func() {
		var memLimitMB = uint64(128)

		BeforeEach(func() {
			memLimitBytes := memLimitMB * 1024 * 1024
			bundleSpec.Windows.Resources = &specs.WindowsResources{
				Memory: &specs.WindowsMemoryResources{
					Limit: &memLimitBytes,
				},
			}
		})

		grabMemory := func(mem int, exitCode int) string {
			cmd := exec.Command(wincBin, "exec", containerId, "c:\\consume.exe", strconv.Itoa(mem*1024*1024))
			stdErr := new(bytes.Buffer)
			session, err := gexec.Start(cmd, GinkgoWriter, io.MultiWriter(stdErr, GinkgoWriter))
			Expect(err).ToNot(HaveOccurred())
			Eventually(session, defaultTimeout*2).Should(gexec.Exit(exitCode))
			return stdErr.String()
		}

		It("is not constrained by smaller memory limit", func() {
			helpers.RunContainer(bundleSpec, bundlePath, containerId)

			helpers.CopyFile(filepath.Join(bundleSpec.Root.Path, "consume.exe"), consumeBin)

			Expect(grabMemory(10, 0)).To(Equal(""))
		})

		It("is constrained by hitting the memory limit", func() {
			helpers.RunContainer(bundleSpec, bundlePath, containerId)

			helpers.CopyFile(filepath.Join(bundleSpec.Root.Path, "consume.exe"), consumeBin)

			Expect(grabMemory(int(memLimitMB), 2)).To(ContainSubstring("fatal error: out of memory"))
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

		It("creates and starts the container and writes the container pid to the specified file", func() {
			helpers.GenerateBundle(bundleSpec, bundlePath)
			_, _, err := helpers.ExecuteNoOutput(exec.Command(wincBin, "run", "-b", bundlePath, "--pid-file", pidFile, "--detach", containerId))
			Expect(err).ToNot(HaveOccurred())

			containerPid := helpers.GetContainerState(containerId).Pid

			pidBytes, err := ioutil.ReadFile(pidFile)
			Expect(err).ToNot(HaveOccurred())
			pid, err := strconv.ParseInt(string(pidBytes), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(int(pid)).To(Equal(containerPid))
		})
	})

	Context("when the --detach flag is passed", func() {
		It("the process runs in the container and returns immediately", func() {
			bundleSpec.Process.Args = []string{"cmd.exe", "/C", "waitfor fivesec /T 5 >NULL & exit /B 0"}
			helpers.GenerateBundle(bundleSpec, bundlePath)
			_, _, err := helpers.ExecuteNoOutput(exec.Command(wincBin, "run", "-b", bundlePath, "--detach", containerId))
			Expect(err).ToNot(HaveOccurred())

			pl := helpers.ContainerProcesses(containerId, "cmd.exe")
			Expect(len(pl)).To(Equal(1))

			containerPid := helpers.GetContainerState(containerId).Pid
			Expect(pl[0].ProcessId).To(Equal(uint32(containerPid)))

			Eventually(func() []hcsshim.ProcessListItem {
				return helpers.ContainerProcesses(containerId, "cmd.exe")
			}, "10s").Should(BeEmpty())
		})
	})

	Context("when the --detach flag is not passed", func() {
		It("the process runs in the container, returns the exit code when the process finishes, and deletes the container", func() {
			bundleSpec.Process.Args = []string{"cmd.exe", "/C", "exit /B 5"}
			helpers.GenerateBundle(bundleSpec, bundlePath)
			_, _, err := helpers.Execute(exec.Command(wincBin, "run", "-b", bundlePath, containerId))
			Expect(err).To(HaveOccurred())
			Expect(helpers.ExitCode(err)).To(Equal(5))

			Expect(helpers.ContainerExists(containerId)).To(BeFalse())
		})

		It("passes stdin through to the process", func() {
			bundleSpec.Process.Args = []string{"findstr", ".*"}
			helpers.GenerateBundle(bundleSpec, bundlePath)
			cmd := exec.Command(wincBin, "run", "-b", bundlePath, containerId)
			cmd.Stdin = strings.NewReader("hey-winc")
			stdOut, _, err := helpers.Execute(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("hey-winc"))
		})

		It("captures the stdout", func() {
			bundleSpec.Process.Args = []string{"cmd.exe", "/C", "echo hey-winc"}
			helpers.GenerateBundle(bundleSpec, bundlePath)
			stdOut, _, err := helpers.Execute(exec.Command(wincBin, "run", "-b", bundlePath, containerId))
			Expect(err).NotTo(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("hey-winc"))
		})

		It("captures the stderr", func() {
			bundleSpec.Process.Args = []string{"cmd.exe", "/C", "echo hey-winc 1>&2"}
			helpers.GenerateBundle(bundleSpec, bundlePath)
			_, stdErr, err := helpers.Execute(exec.Command(wincBin, "run", "-b", bundlePath, containerId))
			Expect(err).NotTo(HaveOccurred())
			Expect(stdErr.String()).To(ContainSubstring("hey-winc"))
		})

		It("captures the CTRL+C", func() {
			bundleSpec.Process.Args = []string{"cmd.exe", "/C", "echo hey-winc & waitfor ever /T 9999"}
			helpers.GenerateBundle(bundleSpec, bundlePath)
			cmd := exec.Command(wincBin, "run", "-b", bundlePath, containerId)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
			}
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Consistently(session).ShouldNot(gexec.Exit(0))
			Eventually(session.Out).Should(gbytes.Say("hey-winc"))
			pl := helpers.ContainerProcesses(containerId, "cmd.exe")
			Expect(len(pl)).To(Equal(1))

			sendCtrlBreak(session)
			Eventually(session).Should(gexec.Exit(1067))
			Expect(helpers.ContainerExists(containerId)).To(BeFalse())
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

			FIt("places the container pid in the specified file", func() {
				bundleSpec.Process.Args = []string{"cmd.exe", "/C", "waitfor ever /T 9999"}
				helpers.GenerateBundle(bundleSpec, bundlePath)
				_, _, err := helpers.ExecuteNoOutput(exec.Command(wincBin, "run", "-b", bundlePath, "--detach", "--pid-file", pidFile, containerId))
				Expect(err).ToNot(HaveOccurred())

				containerPid := helpers.GetContainerState(containerId).Pid

				pidBytes, err := ioutil.ReadFile(pidFile)
				Expect(err).ToNot(HaveOccurred())
				pid, err := strconv.ParseInt(string(pidBytes), 10, 64)
				Expect(err).ToNot(HaveOccurred())
				Expect(int(pid)).To(Equal(containerPid))
			})
		})

		Context("when the '--no-new-keyring' flag is provided", func() {
			It("ignores it and creates and starts a container", func() {
				helpers.GenerateBundle(bundleSpec, bundlePath)
				_, _, err := helpers.Execute(exec.Command(wincBin, "run", "-b", bundlePath, "--detach", "--no-new-keyring", containerId))
				Expect(err).ToNot(HaveOccurred())
				Expect(helpers.ContainerExists(containerId)).To(BeTrue())
			})
		})

		Context("when the container exists", func() {
			BeforeEach(func() {
				helpers.CreateContainer(bundleSpec, bundlePath, containerId)
			})

			AfterEach(func() {
				helpers.DeleteContainer(containerId)
				helpers.DeleteVolume(containerId)
			})

			It("errors", func() {
				_, stdErr, err := helpers.Execute(exec.Command(wincBin, "run", "-b", bundlePath, "--detach", containerId))
				Expect(err).To(HaveOccurred())
				expectedErrorMsg := fmt.Sprintf("container with id already exists: %s", containerId)
				Expect(stdErr.String()).To(ContainSubstring(expectedErrorMsg))
			})
		})

		Context("when the bundlePath is not specified", func() {
			It("uses the current directory as the bundlePath", func() {
				helpers.GenerateBundle(bundleSpec, bundlePath)
				createCmd := exec.Command(wincBin, "run", "--detach", containerId)
				createCmd.Dir = bundlePath
				_, _, err := helpers.Execute(createCmd)
				Expect(err).ToNot(HaveOccurred())
				Expect(helpers.ContainerExists(containerId)).To(BeTrue())
			})
		})
	})
})
