package main_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/volume"
	ps "github.com/mitchellh/go-ps"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("State", func() {
	var (
		stdOut *bytes.Buffer
		stdErr *bytes.Buffer
	)

	BeforeEach(func() {
		stdOut = new(bytes.Buffer)
		stdErr = new(bytes.Buffer)
	})

	Context("given an existing container id", func() {
		var (
			containerId string
			cm          *container.Manager
			actualState *specs.State
			client      *hcs.Client
		)

		BeforeEach(func() {
			containerId = filepath.Base(bundlePath)

			client = &hcs.Client{}
			nm := networkManager(client)
			cm = container.NewManager(client, &volume.Mounter{}, nm, rootPath, bundlePath)

			bundleSpec := runtimeSpecGenerator(createSandbox(rootPath, rootfsPath, containerId), containerId)
			Expect(cm.Create(&bundleSpec)).To(Succeed())
		})

		AfterEach(func() {
			Expect(cm.Delete()).To(Succeed())
			Expect(execute(wincImageBin, "--store", rootPath, "delete", containerId)).To(Succeed())
		})

		Context("when the container has been created", func() {
			It("prints the state of the container to stdout", func() {
				cmd := exec.Command(wincBin, "state", containerId)
				cmd.Stdout = stdOut
				Expect(cmd.Run()).To(Succeed())

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
			session, err := gexec.Start(cmd, stdOut, stdErr)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &hcs.NotFoundError{Id: "doesntexist"}
			Expect(stdErr.String()).To(ContainSubstring(expectedError.Error()))
		})
	})
})
