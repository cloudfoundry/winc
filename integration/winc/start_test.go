package main_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Start", func() {
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
		bundleSpec.Process = &specs.Process{
			Cwd:  "C:\\",
			Args: []string{"cmd.exe", "/C", "echo hello > C:\\out.txt & waitfor ever /t 9999"},
		}
	})

	AfterEach(func() {
		helpers.DeleteContainer(containerId)
		helpers.DeleteVolume(containerId)
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Context("the container has been created but not started", func() {
		BeforeEach(func() {
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
		})

		It("mounts the sandbox.vhdx at C:\\proc\\<pid>\\root", func() {
			helpers.StartContainer(containerId)

			pid := helpers.GetContainerState(containerId).Pid
			Expect(ioutil.WriteFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "test.txt"), []byte("contents"), 0644)).To(Succeed())

			stdOut, _, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "type", "test.txt"}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("contents"))
		})

		It("runs the init process", func() {
			helpers.StartContainer(containerId)

			pl := containerProcesses(containerId, "cmd.exe")
			Expect(len(pl)).To(Equal(1))

			containerPid := helpers.GetContainerState(containerId).Pid
			Expect(pl[0].ProcessId).To(Equal(uint32(containerPid)))

			stdOut, stdErr, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "type c:\\out.txt"}, false)
			Expect(err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())
			Expect(strings.TrimSpace(stdOut.String())).To(Equal("hello"))
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
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "start", "--pid-file", pidFile, containerId))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				containerPid := helpers.GetContainerState(containerId).Pid

				pidBytes, err := ioutil.ReadFile(pidFile)
				Expect(err).ToNot(HaveOccurred())
				pid, err := strconv.ParseInt(string(pidBytes), 10, 64)
				Expect(err).ToNot(HaveOccurred())
				Expect(int(pid)).To(Equal(containerPid))
			})
		})
	})

	Context("the init process has already been started and is still running", func() {
		BeforeEach(func() {
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
			helpers.StartContainer(containerId)
		})

		It("errors", func() {
			_, stdErr, err := helpers.Execute(exec.Command(wincBin, "start", containerId))
			Expect(err).To(HaveOccurred())

			Expect(strings.TrimSpace(stdErr.String())).To(Equal("cannot start a container in the running state"))
		})
	})

	Context("the init process has already been started and has exited", func() {
		BeforeEach(func() {
			bundleSpec.Process = &specs.Process{
				Cwd:  "C:\\",
				Args: []string{"cmd.exe", "/C", "echo hello > C:\\out.txt"},
			}
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
			helpers.StartContainer(containerId)
			theProcessExits(containerId, "cmd.exe")
		})

		It("errors", func() {
			_, stdErr, err := helpers.Execute(exec.Command(wincBin, "start", containerId))
			Expect(err).To(HaveOccurred())

			Expect(strings.TrimSpace(stdErr.String())).To(Equal("cannot start a container in the stopped state"))
		})
	})

	Context("the container has not been created", func() {
		It("errors", func() {
			_, stdErr, err := helpers.Execute(exec.Command(wincBin, "start", containerId))
			Expect(err).To(HaveOccurred())

			Expect(strings.TrimSpace(stdErr.String())).To(Equal(fmt.Sprintf("GetContainerProperties: container not found: %s", containerId)))
		})
	})

	Context("the init process failed to start", func() {
		BeforeEach(func() {
			bundleSpec.Process = &specs.Process{
				Cwd:  "C:\\",
				Args: []string{"cmdf.exe"},
			}
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
		})

		It("errors", func() {
			_, stdErr, err := helpers.Execute(exec.Command(wincBin, "start", containerId))
			Expect(err).To(HaveOccurred())

			Expect(strings.TrimSpace(stdErr.String())).To(ContainSubstring("could not start command 'cmdf.exe'"))
		})
	})
})

func theProcessExits(containerId, image string) {
	exited := false

	for i := 0; i < 5; i++ {
		time.Sleep(time.Duration(i) * time.Second)
		pl := containerProcesses(containerId, image)
		if len(pl) == 0 {
			exited = true
			break
		}
	}
	ExpectWithOffset(1, exited).To(BeTrue())
}
