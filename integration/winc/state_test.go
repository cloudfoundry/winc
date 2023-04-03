package main_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	acl "github.com/hectane/go-acl"
	ps "github.com/mitchellh/go-ps"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/windows"
)

var _ = Describe("State", func() {
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
	})

	AfterEach(func() {
		failed = failed || CurrentGinkgoTestDescription().Failed
		helpers.DeleteContainer(containerId)
		helpers.DeleteVolume(containerId)
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Context("when the container has been created", func() {
		BeforeEach(func() {
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
		})

		It("prints the state of the container to stdout", func() {
			state := helpers.GetContainerState(containerId)

			Expect(state.Status).To(Equal("created"))
			Expect(state.Version).To(Equal(specs.Version))
			Expect(state.ID).To(Equal(containerId))
			Expect(state.Bundle).To(Equal(bundlePath))
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

		It("returns the status as 'running'", func() {
			state := helpers.GetContainerState(containerId)

			Expect(state.Status).To(Equal("running"))
			p, err := ps.FindProcess(state.Pid)
			Expect(err).ToNot(HaveOccurred())
			Expect(p.Executable()).To(Equal("cmd.exe"))
		})
	})

	Context("the process is exectuted with run and is still running", func() {
		BeforeEach(func() {
			bundleSpec.Process = &specs.Process{
				Cwd:  "C:\\",
				Args: []string{"cmd.exe", "/C", "waitfor /t 9999 forever"},
			}

			helpers.GenerateBundle(bundleSpec, bundlePath)
			_, _, err := helpers.Execute(exec.Command(wincBin, "run", "-b", bundlePath, "--detach", containerId))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the status as 'running'", func() {
			state := helpers.GetContainerState(containerId)

			Expect(state.Status).To(Equal("running"))
			p, err := ps.FindProcess(state.Pid)
			Expect(err).ToNot(HaveOccurred())
			Expect(p.Executable()).To(Equal("cmd.exe"))
		})
	})

	Context("the init process has already been started and has exited", func() {
		BeforeEach(func() {
			bundleSpec.Process = &specs.Process{
				Cwd:  "C:\\",
				Args: []string{"cmd.exe", "/C", "echo hello"},
			}

			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
			helpers.StartContainer(containerId)
			helpers.TheProcessExits(containerId, "cmd.exe")
		})

		It("returns the status as 'stopped'", func() {
			state := helpers.GetContainerState(containerId)

			Expect(state.Status).To(Equal("stopped"))
		})
	})

	Context("the init process failed to start", func() {
		BeforeEach(func() {
			bundleSpec.Process = &specs.Process{
				Cwd:  "C:\\",
				Args: []string{"cmdf.exe"},
			}
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
			_, stdErr, err := helpers.Execute(exec.Command(wincBin, "start", containerId))
			Expect(err).To(HaveOccurred())
			Expect(strings.TrimSpace(stdErr.String())).To(ContainSubstring("could not start command 'cmdf.exe'"))
		})

		It("returns the status as 'stopped'", func() {
			state := helpers.GetContainerState(containerId)

			Expect(state.Status).To(Equal("stopped"))
		})
	})

	Context("given a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "state", "doesntexist")
			stdOut, stdErr, err := helpers.Execute(cmd)
			Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())

			Expect(stdErr.String()).To(ContainSubstring("container not found: doesntexist"))
		})
	})
})
