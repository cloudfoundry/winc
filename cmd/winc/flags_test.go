package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	. "code.cloudfoundry.org/winc/cmd/winc"
	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/mounter"
	"code.cloudfoundry.org/winc/sandbox"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Flags", func() {
	var (
		args             []string
		err              error
		session          *gexec.Session
		expectedExitCode int
		containerId      string
	)

	BeforeEach(func() {
		containerId = strconv.Itoa(rand.Int())
		bundlePath = filepath.Join(depotDir, containerId)

		Expect(os.MkdirAll(bundlePath, 0755)).To(Succeed())
		args = []string{}
		expectedExitCode = 0
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		wincCmd := exec.Command(wincBin, args...)
		session, err = gexec.Start(wincCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(session).Should(gexec.Exit(expectedExitCode))
	})

	Context("when passed a nonexistent flag", func() {
		BeforeEach(func() {
			args = []string{"--nonexistent"}
			expectedExitCode = 1
		})

		It("prints a message saying the flag does not exist", func() {
			Eventually(session.Err).Should(gbytes.Say("flag provided but not defined: -nonexistent"))
		})
	})

	Context("when passed no flags", func() {
		It("prints a help message", func() {
			Eventually(session.Out).Should(gbytes.Say("NAME:\n.*winc.exe - Open Container Initiative runtime for Windows"))
		})
	})

	Context("when passed '--help'", func() {
		BeforeEach(func() {
			args = []string{"--help"}
		})

		It("prints a help message", func() {
			Eventually(session.Out).Should(gbytes.Say("NAME:\n.*winc.exe - Open Container Initiative runtime for Windows"))
		})
	})

	Context("when passed '-h'", func() {
		BeforeEach(func() {
			args = []string{"-h"}
		})

		It("prints a help message", func() {
			Eventually(session.Out).Should(gbytes.Say("NAME:\n.*winc.exe - Open Container Initiative runtime for Windows"))
		})
	})

	Context("when passed '--log'", func() {
		var logFile string

		BeforeEach(func() {
			f, err := ioutil.TempFile("", "winc.log")
			Expect(err).ToNot(HaveOccurred())
			Expect(f.Close()).To(Succeed())
			logFile = f.Name()
			args = []string{"--log", logFile}
		})

		AfterEach(func() {
			Expect(os.RemoveAll(logFile)).To(Succeed())
		})

		It("accepts the flag and prints the --log flag usage", func() {
			Eventually(session.Out).Should(gbytes.Say("GLOBAL OPTIONS:(.|\n)*--log value"))
		})

		Context("when the winc command logs non error messages", func() {
			var containerId string

			BeforeEach(func() {
				bundleSpec := runtimeSpecGenerator(rootfsPath)
				config, err := json.Marshal(&bundleSpec)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0755)).To(Succeed())

				containerId = filepath.Base(bundlePath)

				args = []string{"--log", logFile, "create", containerId, "-b", bundlePath}
			})

			It("does not log anything", func() {
				log, err := ioutil.ReadFile(logFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(log).To(BeEmpty())

				Expect(session.Out.Contents()).To(BeEmpty())
			})

			AfterEach(func() {
				client := &hcsclient.HCSClient{}
				sm := sandbox.NewManager(client, &mounter.Mounter{}, depotDir, containerId)
				nm := networkManager(client)
				cm := container.NewManager(client, sm, nm, containerId)
				Expect(cm.Delete()).To(Succeed())
			})

			Context("when the --debug flag is set", func() {
				BeforeEach(func() {
					args = []string{"--log", logFile, "--debug", "create", containerId, "-b", bundlePath}
				})

				It("logs to the log file instead of stdout", func() {
					log, err := ioutil.ReadFile(logFile)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(log)).To(ContainSubstring(fmt.Sprintf("containerId=%s", containerId)))

					Expect(session.Out.Contents()).To(BeEmpty())
				})
			})
		})

		Context("when the winc command errors", func() {
			BeforeEach(func() {
				args = append(args, "create", "nonexistent")
				expectedExitCode = 1
			})

			It("logs the error to the specified log file and still prints the final error to stderr", func() {
				log, err := ioutil.ReadFile(logFile)
				Expect(err).ToNot(HaveOccurred())
				expectedError := &MissingBundleConfigError{}
				Eventually(session.Err).Should(gbytes.Say(expectedError.Error()))
				expectedLogContents := strings.Trim(string(session.Err.Contents()), "\n")
				Expect(string(log)).To(ContainSubstring(expectedLogContents))
			})
		})
	})

	Context("when passed '--log-format'", func() {
		BeforeEach(func() {
			args = []string{"--log-format", "text"}
		})

		It("accepts the flag and prints the --log-format flag usage", func() {
			Eventually(session.Out).Should(gbytes.Say("GLOBAL OPTIONS:(.|\n)*--log-format value"))
		})

		Context("when provided an invalid log format", func() {
			BeforeEach(func() {
				args = []string{"--log-format", "invalid"}
				expectedExitCode = 1
			})

			It("errors", func() {
				expectedError := &InvalidLogFormatError{Format: "invalid"}
				Eventually(session.Err).Should(gbytes.Say(expectedError.Error()))
			})
		})
	})

	Context("when passed '--newuidmap'", func() {
		BeforeEach(func() {
			args = []string{"--newuidmap", "foo"}
		})

		It("accepts the flag and prints the --newuidmap flag usage", func() {
			Eventually(session.Out).Should(gbytes.Say("GLOBAL OPTIONS:(.|\n)*--newuidmap value"))
		})
	})

	Context("when passed '--newgidmap'", func() {
		BeforeEach(func() {
			args = []string{"--newgidmap", "foo"}
		})

		It("accepts the flag and prints the --newgidmap flag usage", func() {
			Eventually(session.Out).Should(gbytes.Say("GLOBAL OPTIONS:(.|\n)*--newgidmap value"))
		})
	})
})
