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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/windows"
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
			Args: []string{"cmd.exe", "/C", "echo hello > C:\\out.txt "},
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

		It("runs the init process", func() {
			helpers.StartContainer(containerId)
			theProcessExits(containerId, "cmd.exe")

			stdOut, stdErr, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "type c:\\out.txt"}, false)
			Expect(err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())
			Expect(strings.TrimSpace(stdOut.String())).To(Equal("hello"))
		})
	})

	FContext("when the duplicate-handle-file option is passed", func() {
		var tmpFile *os.File

		BeforeEach(func() {
			var err error

			tmpFile, err = ioutil.TempFile("", "test-dup-handle")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			tmpFile.Close()
			Expect(os.RemoveAll(tmpFile.Name())).To(Succeed())
		})

		Context("when child process has exited", func() {
			BeforeEach(func() {
				bundleSpec.Process.Args = []string{"cmd.exe", "/C", "exit /B 8"}
				helpers.CreateContainer(bundleSpec, bundlePath, containerId)
			})

			It("attaches handle to the parent process, referenced by the duplicate handle file", func() {
				_, _, err := helpers.Execute(exec.Command(wincBin, "start", "--duplicate-handle-file", tmpFile.Name(), containerId))
				Expect(err).NotTo(HaveOccurred())

				handleAddressContents, err := ioutil.ReadFile(tmpFile.Name())
				Expect(err).NotTo(HaveOccurred())

				exitCode := waitForHandleToExit(string(handleAddressContents))
				Expect(exitCode).To(Equal(uint32(8)))
			})
		})

		Context("when child process is still running", func() {
			BeforeEach(func() {
				bundleSpec.Process.Args = []string{"cmd.exe", "/C", "waitfor /t 1 foronesecond"}
				helpers.CreateContainer(bundleSpec, bundlePath, containerId)
			})

			It("attaches handle to the parent process, referenced by the duplicate handle file", func() {
				_, _, err := helpers.Execute(exec.Command(wincBin, "start", "--duplicate-handle-file", tmpFile.Name(), containerId))
				Expect(err).NotTo(HaveOccurred())

				handleAddressContents, err := ioutil.ReadFile(tmpFile.Name())
				Expect(err).NotTo(HaveOccurred())

				exitCode := waitForHandleToExit(string(handleAddressContents))
				Expect(exitCode).To(Equal(uint32(1)))
			})
		})
	})

	Context("the init process has already been started and is still running", func() {
		BeforeEach(func() {
			bundleSpec.Process = &specs.Process{
				Cwd:  "C:\\",
				Args: []string{"cmd.exe", "/C", "waitfor /t 9999 forever"},
			}

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
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
			helpers.StartContainer(containerId)
			theProcessExits(containerId, "cmd.exe")
		})

		It("errors", func() {
			_, stdErr, err := helpers.Execute(exec.Command(wincBin, "start", containerId))
			Expect(err).To(HaveOccurred())

			Expect(strings.TrimSpace(stdErr.String())).To(Equal("cannot start a container in the exited state"))
		})
	})

	Context("the container has not been created", func() {
		It("errors", func() {
			_, stdErr, err := helpers.Execute(exec.Command(wincBin, "start", containerId))
			Expect(err).To(HaveOccurred())

			Expect(strings.TrimSpace(stdErr.String())).To(Equal(fmt.Sprintf("container not found: %s", containerId)))
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

func waitForHandleToExit(handleAddressContents string) uint32 {
	handleAddress, err := strconv.Atoi(handleAddressContents)
	Expect(err).NotTo(HaveOccurred())

	handle := syscall.Handle(handleAddress)

	_, err = syscall.WaitForSingleObject(handle, windows.INFINITE)
	Expect(err).NotTo(HaveOccurred())
	defer syscall.CloseHandle(handle)

	var exitCode uint32
	err = syscall.GetExitCodeProcess(handle, &exitCode)
	Expect(err).NotTo(HaveOccurred())

	return exitCode
}

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
