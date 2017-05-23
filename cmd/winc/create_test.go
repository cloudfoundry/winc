package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/sandbox"

	. "code.cloudfoundry.org/winc/cmd/winc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Create", func() {
	const createTimeout = time.Second * 2

	var (
		config      []byte
		containerId string
		client      hcsclient.Client
		cm          container.ContainerManager
		bundleSpec  specs.Spec
		err         error
	)

	BeforeEach(func() {
		containerId = filepath.Base(bundlePath)

		bundleSpec = runtimeSpecGenerator(rootfsPath)
		config, err = json.Marshal(&bundleSpec)
		Expect(err).NotTo(HaveOccurred())

		client = &hcsclient.HCSClient{}
		sm := sandbox.NewManager(client, bundlePath)
		cm = container.NewManager(client, sm, containerId)
	})

	JustBeforeEach(func() {
		Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0755)).To(Succeed())
	})

	Context("when provided valid arguments", func() {
		AfterEach(func() {
			Expect(cm.Delete()).To(Succeed())
		})

		It("creates and starts a container", func() {
			cmd := exec.Command(wincBin, "create", "-b", bundlePath, containerId)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session, createTimeout).Should(gexec.Exit(0))

			Expect(containerExists(containerId)).To(BeTrue())

			sandboxVHDX := filepath.Join(bundlePath, "sandbox.vhdx")
			_, err = os.Stat(sandboxVHDX)
			Expect(err).ToNot(HaveOccurred())

			cmd = exec.Command("powershell", "-Command", "Test-VHD", sandboxVHDX)
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session, time.Second*3).Should(gexec.Exit(0))

			sandboxInitialized := filepath.Join(bundlePath, "initialized")
			_, err = os.Stat(sandboxInitialized)
			Expect(err).ToNot(HaveOccurred())

			layerChainPath := filepath.Join(bundlePath, "layerchain.json")
			_, err = os.Stat(layerChainPath)
			Expect(err).ToNot(HaveOccurred())

			layerChain, err := ioutil.ReadFile(layerChainPath)
			Expect(err).ToNot(HaveOccurred())

			layers := []string{}
			err = json.Unmarshal(layerChain, &layers)
			Expect(err).ToNot(HaveOccurred())

			Expect(layers[0]).To(Equal(rootfsPath))

			state, err := cm.State()
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Pid).ToNot(Equal(-1))
		})

		Context("when the bundle path is not provided", func() {
			It("uses the current directory as the bundle path", func() {
				cmd := exec.Command(wincBin, "create", containerId)
				cmd.Dir = bundlePath
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				Eventually(session, createTimeout).Should(gexec.Exit(0))

				state, err := cm.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(state.Pid).ToNot(Equal(-1))
			})
		})

		Context("when the '--pid-file' flag is provided", func() {
			var pidFile string

			BeforeEach(func() {
				pidFile = filepath.Join(os.TempDir(), string(time.Now().UnixNano()))
			})

			AfterEach(func() {
				Expect(os.RemoveAll(pidFile)).To(Succeed())
			})

			It("creates and starts the container and writes the container pid to the specified file", func() {
				cmd := exec.Command(wincBin, "create", "-b", bundlePath, "--pid-file", pidFile, containerId)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				Eventually(session, createTimeout).Should(gexec.Exit(0))

				state, err := cm.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(state.Pid).ToNot(Equal(-1))

				pidBytes, err := ioutil.ReadFile(pidFile)
				Expect(err).ToNot(HaveOccurred())
				pid, err := strconv.ParseInt(string(pidBytes), 10, 64)
				Expect(err).ToNot(HaveOccurred())
				Expect(int(pid)).To(Equal(state.Pid))
			})
		})

		Context("when the '--no-new-keyring' flag is provided", func() {
			It("ignores it and creates and starts a container", func() {
				cmd := exec.Command(wincBin, "create", containerId, "--no-new-keyring")
				cmd.Dir = bundlePath
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				Eventually(session, createTimeout).Should(gexec.Exit(0))

				state, err := cm.State()
				Expect(err).ToNot(HaveOccurred())
				Expect(state.Pid).ToNot(Equal(-1))
			})
		})
	})

	Context("when provided a container id that already exists", func() {
		BeforeEach(func() {
			Expect(cm.Create(rootfsPath)).To(Succeed())
		})

		AfterEach(func() {
			Expect(cm.Delete()).To(Succeed())
		})

		It("errors", func() {
			cmd := exec.Command(wincBin, "create", "-b", bundlePath, containerId)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &hcsclient.AlreadyExistsError{Id: containerId}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))
		})
	})

	Context("when the bundle directory name and container id do not match", func() {
		It("errors and does not create the container", func() {
			containerId = "doesnotmatchbundle"
			cmd := exec.Command(wincBin, "create", "-b", bundlePath, containerId)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &hcsclient.InvalidIdError{Id: containerId}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))

			Expect(containerExists(containerId)).To(BeFalse())
		})
	})

	Context("when provided a nonexistent bundle directory", func() {
		It("errors and does not create the container", func() {
			cmd := exec.Command(wincBin, "create", "-b", "idontexist", containerId)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &MissingBundleError{BundlePath: "idontexist"}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))

			Expect(containerExists(containerId)).To(BeFalse())
		})
	})

	Context("when provided a bundle with a config.json that is invalid JSON", func() {
		BeforeEach(func() {
			config = []byte("{")
		})

		It("errors and does not create the container", func() {
			wincCmd := exec.Command(wincBin, "create", "-b", bundlePath, containerId)
			session, err := gexec.Start(wincCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &BundleConfigInvalidJSONError{}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))

			Expect(containerExists(containerId)).To(BeFalse())
		})
	})

	Context("when provided a bundle with a config.json that does not conform to the runtime spec", func() {
		It("errors and does not create the container", func() {
			bundleSpec.Platform.OS = ""
			config, err := json.Marshal(&bundleSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0755)).To(Succeed())
			cmd := exec.Command(wincBin, "create", "-b", bundlePath, containerId)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &BundleConfigValidationError{}
			Expect(session.Err).To(gbytes.Say(expectedError.Error()))

			Expect(containerExists(containerId)).To(BeFalse())
		})
	})
})
