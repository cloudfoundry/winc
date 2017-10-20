package main_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	ps "github.com/mitchellh/go-ps"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("State", func() {
	Context("given an existing container id", func() {
		var (
			containerId string
			actualState *specs.State
		)

		BeforeEach(func() {
			containerId = filepath.Base(bundlePath)

			bundleSpec := runtimeSpecGenerator(createSandbox(imageStore, rootfsPath, containerId))
			config, err := json.Marshal(&bundleSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0666)).To(Succeed())
			_, _, err = execute(exec.Command(wincBin, "create", "-b", bundlePath, containerId))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, _, err := execute(exec.Command(wincBin, "delete", containerId))
			Expect(err).ToNot(HaveOccurred())
			_, _, err = execute(exec.Command(wincImageBin, "--store", imageStore, "delete", containerId))
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the container has been created", func() {
			It("prints the state of the container to stdout", func() {
				stdOut, _, err := execute(exec.Command(wincBin, "state", containerId))
				Expect(err).ToNot(HaveOccurred())

				actualState = &specs.State{}
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
			stdErr := new(bytes.Buffer)
			session, err := gexec.Start(cmd, GinkgoWriter, io.MultiWriter(stdErr, GinkgoWriter))
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(stdErr.String()).To(ContainSubstring("container not found: doesntexist"))
		})
	})
})
