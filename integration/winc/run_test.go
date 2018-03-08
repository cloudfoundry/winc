package main_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/Microsoft/hcsshim"
	acl "github.com/hectane/go-acl"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/windows"
)

var _ = Describe("Run", func() {
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
		mountSrc := filepath.Dir(sleepBin)
		bundleSpec.Mounts = []specs.Mount{{Source: mountSrc, Destination: "C:\\mountSomewhereElse"}}
		Expect(ioutil.WriteFile(filepath.Join(mountSrc, "hi.txt"), []byte("hello"), 0644)).To(Succeed())
		Expect(acl.Apply(mountSrc, false, false, acl.GrantName(windows.GENERIC_ALL, "Everyone"))).To(Succeed())
		//		bundleSpec.Process.Args = []string{filepath.Join("c:\\mountSomewhereElse", "sleep.exe")}
		bundleSpec.Process.Args = []string{filepath.Join("sleep.exe")}
		fmt.Println(mountSrc)
	})

	AfterEach(func() {
		//		helpers.DeleteContainer(containerId)
		//		helpers.DeleteVolume(containerId)
		//		Expect(os.RemoveAll(bundlePath)).To(Succeed())
		fmt.Println(containerId)
	})

	FIt("creates a container and runs the init process", func() {
		helpers.GenerateBundle(bundleSpec, bundlePath)
		//	_, _, err := helpers.Execute(exec.Command(wincBin, "--debug", "create", "-b", bundlePath, containerId))
		_, _, err := helpers.Execute(exec.Command(wincBin, "--debug", "run", "-b", bundlePath, "--detach", containerId))
		Expect(err).ToNot(HaveOccurred())

		Expect(helpers.ContainerExists(containerId)).To(BeTrue())

		pl := containerProcesses(containerId, "sleep.exe")
		Expect(len(pl)).To(Equal(1))

		containerPid := helpers.GetContainerState(containerId).Pid
		Expect(isParentOf(containerPid, int(pl[0].ProcessId))).To(BeTrue())
	})

	Context("when the --detach flag is passed", func() {
		It("the process runs in the container and returns immediately", func() {
			bundleSpec.Process.Args = []string{"cmd.exe", "/C", "waitfor fivesec /T 5 >NULL & exit /B 0"}
			helpers.GenerateBundle(bundleSpec, bundlePath)
			_, _, err := helpers.Execute(exec.Command(wincBin, "run", "-b", bundlePath, "--detach", containerId))
			Expect(err).ToNot(HaveOccurred())

			pl := containerProcesses(containerId, "cmd.exe")
			Expect(len(pl)).To(Equal(1))

			containerPid := helpers.GetContainerState(containerId).Pid
			Expect(isParentOf(containerPid, int(pl[0].ProcessId))).To(BeTrue())

			Eventually(func() []hcsshim.ProcessListItem {
				return containerProcesses(containerId, "cmd.exe")
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
			bundleSpec.Process.Args = []string{"C:\\temp\\read.exe"}
			bundleSpec.Mounts = []specs.Mount{
				{
					Source:      filepath.Dir(readBin),
					Destination: "C:\\temp",
				},
			}
			helpers.GenerateBundle(bundleSpec, bundlePath)
			cmd := exec.Command(wincBin, "run", "-b", bundlePath, containerId)
			cmd.Stdin = strings.NewReader("hey-winc\n")
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
			pl := containerProcesses(containerId, "cmd.exe")
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

			It("places the container pid in the specified file", func() {
				bundleSpec.Process.Args = []string{"cmd.exe", "/C", "waitfor ever /T 9999"}
				helpers.GenerateBundle(bundleSpec, bundlePath)
				_, _, err := helpers.Execute(exec.Command(wincBin, "run", "-b", bundlePath, "--detach", "--pid-file", pidFile, containerId))
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
