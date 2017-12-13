package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	helpers "code.cloudfoundry.org/winc/cmd/helpers"
	acl "github.com/hectane/go-acl"
	ps "github.com/mitchellh/go-ps"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/windows"
)

var _ = Describe("State", func() {
	Context("given an existing container id", func() {
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

		Context("when the container has been created", func() {
			It("prints the state of the container to stdout", func() {
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "state", containerId))
				Expect(err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())

				actualState := &specs.State{}
				Expect(json.Unmarshal(stdOut.Bytes(), actualState)).To(Succeed())

				Expect(actualState.Status).To(Equal("created"))
				Expect(actualState.Version).To(Equal(specs.Version))
				Expect(actualState.ID).To(Equal(containerId))
				Expect(actualState.Bundle).To(Equal(bundlePath))

				p, err := ps.FindProcess(actualState.Pid)
				Expect(err).ToNot(HaveOccurred())
				Expect(p.Executable()).To(Equal("wininit.exe"))
			})
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
