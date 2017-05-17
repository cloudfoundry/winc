package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/winc/container"
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
			containerId string
			bundlePath  string
		)

		BeforeEach(func() {
			var err error
			bundlePath, err = ioutil.TempDir("", "winccontainer")
			Expect(err).To(Succeed())

			rootfsPath, present := os.LookupEnv("WINC_TEST_ROOTFS")
			Expect(present).To(BeTrue())
			containerId = filepath.Base(bundlePath)

			Expect(container.Create(rootfsPath, bundlePath, containerId)).To(Succeed())

			query := hcsshim.ComputeSystemQuery{
				Owners: []string{"winc"},
				IDs:    []string{containerId},
			}
			containers, err := hcsshim.GetContainers(query)
			Expect(err).ToNot(HaveOccurred())
			Expect(containers).To(HaveLen(1))
		})

		AfterEach(func() {
			Expect(container.Delete(containerId)).To(Succeed())

			_, err := os.Stat(bundlePath)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("prints the state of the container to stdout", func() {
			cmd := exec.Command(wincBin, "state", containerId)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			expectedState := specs.State{
				Version: specs.Version,
				ID:      containerId,
				Status:  "created",
				Bundle:  bundlePath,
			}

			var outState specs.State
			Expect(json.Unmarshal(session.Out.Contents(), &outState)).To(Succeed())
			Expect(outState).To(Equal(expectedState))
		})
	})

	Context("given a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "state", "doesntexist")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &container.ContainerNotFoundError{Id: "doesntexist"}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))
		})
	})
})
