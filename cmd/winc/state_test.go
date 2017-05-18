package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/sandbox"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("State", func() {
	Context("given an existing container id", func() {
		var (
			containerId   string
			bundlePath    string
			cm            container.ContainerManager
			expectedState *specs.State
			actualState   *specs.State
		)

		BeforeEach(func() {
			var err error
			bundlePath, err = ioutil.TempDir("", "winccontainer")
			Expect(err).To(Succeed())

			rootfsPath, present := os.LookupEnv("WINC_TEST_ROOTFS")
			Expect(present).To(BeTrue())
			containerId = filepath.Base(bundlePath)

			client := hcsclient.HCSClient{}
			sm := sandbox.NewManager(&client, bundlePath)
			cm = container.NewManager(&client, sm, containerId)

			Expect(cm.Create(rootfsPath)).To(Succeed())

			query := hcsshim.ComputeSystemQuery{
				Owners: []string{"winc"},
				IDs:    []string{containerId},
			}
			containers, err := hcsshim.GetContainers(query)
			Expect(err).ToNot(HaveOccurred())
			Expect(containers).To(HaveLen(1))

			expectedState = &specs.State{
				Version: specs.Version,
				ID:      containerId,
				Bundle:  bundlePath,
			}
		})

		JustBeforeEach(func() {
			cmd := exec.Command(wincBin, "state", containerId)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			actualState = &specs.State{}
			Expect(json.Unmarshal(session.Out.Contents(), actualState)).To(Succeed())
			Expect(actualState).To(Equal(expectedState))
		})

		AfterEach(func() {
			Expect(cm.Delete()).To(Succeed())

			_, err := os.Stat(bundlePath)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		Context("when the container has been created", func() {
			BeforeEach(func() {
				expectedState.Status = "created"
			})

			It("prints the state of the container to stdout", func() {
				Expect(actualState).To(Equal(expectedState))
			})
		})

		XContext("when the container is running", func() {
			BeforeEach(func() {
				expectedState.Status = "running"
			})

			It("prints the state of the container to stdout", func() {
				Expect(actualState).To(Equal(expectedState))
			})
		})

		XContext("when the container is stopped", func() {
			BeforeEach(func() {
				expectedState.Status = "stopped"
			})

			It("prints the state of the container to stdout", func() {
				Expect(actualState).To(Equal(expectedState))
			})
		})
	})

	Context("given a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "state", "doesntexist")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &hcsclient.NotFoundError{Id: "doesntexist"}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))
		})
	})
})
