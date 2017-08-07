package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "code.cloudfoundry.org/winc/cmd/winc"
	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/volume"
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
		containerId = filepath.Base(bundlePath)

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
			BeforeEach(func() {
				bundleSpec := runtimeSpecGenerator(createSandbox(rootPath, rootfsPath, containerId), containerId)
				config, err := json.Marshal(&bundleSpec)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0755)).To(Succeed())

				args = []string{"--log", logFile, "create", containerId, "-b", bundlePath}
			})

			AfterEach(func() {
				client := &hcs.Client{}
				nm := networkManager(client)
				cm := container.NewManager(client, &volume.Mounter{}, nm, rootPath, containerId)
				Expect(cm.Delete()).To(Succeed())
				Expect(execute(wincImageBin, "--store", rootPath, "delete", containerId)).To(Succeed())
			})

			It("does not log anything", func() {
				log, err := ioutil.ReadFile(logFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(log).To(BeEmpty())

				Expect(session.Out.Contents()).To(BeEmpty())
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

	Context("when passed '--root'", func() {
		var (
			storePath string
			cm        *container.Manager
		)

		BeforeEach(func() {
			storePath, err = ioutil.TempDir("", "wincroot")
			Expect(err).ToNot(HaveOccurred())

			client := &hcs.Client{}
			nm := networkManager(client)
			cm = container.NewManager(client, &volume.Mounter{}, nm, storePath, containerId)

			bundleSpec := runtimeSpecGenerator(createSandbox(storePath, rootfsPath, containerId), containerId)
			config, err := json.Marshal(&bundleSpec)
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0755)).To(Succeed())

			args = []string{"--root", storePath, "create", containerId, "-b", bundlePath}
		})

		AfterEach(func() {
			Expect(cm.Delete()).To(Succeed())
			Expect(execute(wincImageBin, "--store", storePath, "delete", containerId)).To(Succeed())
			Expect(os.RemoveAll(storePath)).To(Succeed())
		})

		It("is able to create a container with the specified root", func() {
			state, err := cm.State()
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Pid).ToNot(Equal(-1))
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
