package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	helpers "code.cloudfoundry.org/winc/cmd/helpers"
	ps "github.com/mitchellh/go-ps"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func generateBundle(bundleSpec specs.Spec, bundlePath, id string) {
	config, err := json.Marshal(&bundleSpec)
	Expect(err).NotTo(HaveOccurred())
	configFile := filepath.Join(bundlePath, "config.json")
	Expect(ioutil.WriteFile(configFile, config, 0666)).To(Succeed())
}

var _ = Describe("Create", func() {
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
	})

	AfterEach(func() {
		helpers.DeleteSandbox(wincImageBin, imageStore, containerId)
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Context("when provided valid arguments", func() {
		AfterEach(func() {
			helpers.DeleteContainer(wincBin, containerId)
		})

		It("creates and starts a container", func() {
			generateBundle(bundleSpec, bundlePath, containerId)
			stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, containerId))
			Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
			Expect(helpers.ContainerExists(containerId)).To(BeTrue())
			Expect(ps.FindProcess(helpers.GetContainerState(wincBin, containerId).Pid)).ToNot(BeNil())
		})

		It("mounts the sandbox.vhdx at C:\\proc\\<pid>\\root", func() {
			generateBundle(bundleSpec, bundlePath, containerId)
			stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, containerId))
			Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

			pid := helpers.GetContainerState(wincBin, containerId).Pid
			Expect(ioutil.WriteFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "test.txt"), []byte("contents"), 0644)).To(Succeed())

			stdOut, stdErr, err = helpers.ExecInContainer(wincBin, containerId, []string{"cmd.exe", "/C", "type", "test.txt"}, false)
			Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

			Expect(stdOut.String()).To(ContainSubstring("contents"))
		})

		Context("when the bundle path is not provided", func() {
			It("uses the current directory as the bundle path", func() {
				generateBundle(bundleSpec, bundlePath, containerId)
				createCmd := exec.Command(wincBin, "create", containerId)
				createCmd.Dir = bundlePath
				stdOut, stdErr, err := helpers.Execute(createCmd)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				Expect(helpers.ContainerExists(containerId)).To(BeTrue())
			})
		})

		Context("when the bundle path ends with a \\", func() {
			It("creates a container sucessfully", func() {
				generateBundle(bundleSpec, bundlePath, containerId)
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath+"\\", containerId))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				Expect(helpers.ContainerExists(containerId)).To(BeTrue())
			})
		})

		Context("when the '--pid-file' flag is provided", func() {
			var pidFile string

			BeforeEach(func() {
				f, err := ioutil.TempFile("", "pid")
				Expect(err).ToNot(HaveOccurred())
				Expect(f.Close()).To(Succeed())
				pidFile = f.Name()
			})

			AfterEach(func() {
				Expect(os.RemoveAll(pidFile)).To(Succeed())
			})

			It("creates and starts the container and writes the container pid to the specified file", func() {
				generateBundle(bundleSpec, bundlePath, containerId)
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, "--pid-file", pidFile, containerId))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				containerPid := helpers.GetContainerState(wincBin, containerId).Pid

				pidBytes, err := ioutil.ReadFile(pidFile)
				Expect(err).ToNot(HaveOccurred())
				pid, err := strconv.ParseInt(string(pidBytes), 10, 64)
				Expect(err).ToNot(HaveOccurred())
				Expect(int(pid)).To(Equal(containerPid))
			})
		})

		Context("when the '--no-new-keyring' flag is provided", func() {
			It("ignores it and creates and starts a container", func() {
				generateBundle(bundleSpec, bundlePath, containerId)
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, "--no-new-keyring", containerId))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				Expect(helpers.ContainerExists(containerId)).To(BeTrue())
			})
		})

		Context("when the bundle config.json specifies a hostname", func() {
			BeforeEach(func() {
				bundleSpec.Hostname = "some-random-hostname"
			})

			It("sets it as the container hostname", func() {
				generateBundle(bundleSpec, bundlePath, containerId)
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, "--no-new-keyring", containerId))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				stdOut, stdErr, err = helpers.ExecInContainer(wincBin, containerId, []string{"hostname"}, false)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
				Expect(strings.TrimSpace(stdOut.String())).To(Equal("some-random-hostname"))
			})
		})

		Context("when the bundle config.json specifies bind mounts", func() {
			// 	var (
			// 		mountSource string
			// 		mountDest   string
			// 	)

			BeforeEach(func() {
				// 		var err error
				// 		mountSource, err = ioutil.TempDir("", "mountsource")
				// 		Expect(err).ToNot(HaveOccurred())
				// 		Expect(ioutil.WriteFile(filepath.Join(mountSource, "sentinel"), []byte("hello"), 0644)).To(Succeed())

				// 		mountDest = "C:\\mountdest"

				// 		mount := specs.Mount{Destination: mountDest, Source: mountSource}
				// 		bundleSpec.Mounts = []specs.Mount{mount}
			})

			AfterEach(func() {
				// 		Expect(os.RemoveAll(mountSource)).To(Succeed())
			})

			// 	It("creates a container with the specified directories as mounts", func() {
			// 		stdOut, _, err := execute(exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "type", filepath.Join(mountDest, "sentinel")))
			// 		Expect(err).ToNot(HaveOccurred())
			// 		Expect(stdOut.String()).To(ContainSubstring("hello"))
			// 	})

			// 	It("the mounted directories are read only", func() {
			// 		cmd := exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "echo hello > "+filepath.Join(mountDest, "sentinel2"))
			// 		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			// 		Expect(err).ToNot(HaveOccurred())
			// 		Eventually(session).Should(gexec.Exit(1))
			// 	})

			// 	Context("when the destination is /tmp/", func() {
			// 		BeforeEach(func() {
			// 			mountDest = "/tmp/mountdest"

			// 			mount := specs.Mount{Destination: mountDest, Source: mountSource}
			// 			bundleSpec.Mounts = []specs.Mount{mount}
			// 		})

			// 		It("mounts the specified directories", func() {
			// 			stdOut, _, err := execute(exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "type", filepath.Join(mountDest, "sentinel")))
			// 			Expect(err).ToNot(HaveOccurred())
			// 			Expect(stdOut.String()).To(ContainSubstring("hello"))
			// 		})

			// 		Context("when calling the mounted executable", func() {
			// 			BeforeEach(func() {
			// 				Expect(copy(filepath.Join(mountSource, "cmd.exe"), "C:\\Windows\\System32\\cmd.exe")).To(Succeed())

			// 			})
			// 			Context("when using the windows path", func() {
			// 				It("mounts the specified directories", func() {
			// 					stdOut, _, err := execute(exec.Command(wincBin, "exec", containerId, filepath.Join(mountDest, "cmd"), "/C", "type", filepath.Join(mountDest, "sentinel")))
			// 					Expect(err).ToNot(HaveOccurred())
			// 					Expect(stdOut.String()).To(ContainSubstring("hello"))
			// 				})
			// 			})
			// 			Context("when using the unix path", func() {
			// 				It("mounts the specified directories", func() {
			// 					stdOut, _, err := execute(exec.Command(wincBin, "exec", containerId, mountDest+"/cmd", "/C", "type", filepath.Join(mountDest, "sentinel")))
			// 					Expect(err).ToNot(HaveOccurred())
			// 					Expect(stdOut.String()).To(ContainSubstring("hello"))
			// 				})
			// 			})
			// 		})
		})

		// 	Context("when a file is supplied as a mount", func() {
		// 		var (
		// 			logFile   string
		// 			mountFile string
		// 		)

		// 		BeforeEach(func() {
		// 			l, err := ioutil.TempFile("", "winc.log")
		// 			Expect(err).ToNot(HaveOccurred())
		// 			Expect(l.Close()).To(Succeed())
		// 			logFile = l.Name()

		// 			m, err := ioutil.TempFile("", "mountfile")
		// 			Expect(err).ToNot(HaveOccurred())
		// 			Expect(m.Close()).To(Succeed())
		// 			mountFile = m.Name()

		// 			bundleSpec.Mounts = append(bundleSpec.Mounts, specs.Mount{
		// 				Source:      mountFile,
		// 				Destination: "C:\\foobar",
		// 			})

		// 			createCmd = exec.Command(wincBin, "--debug", "--log", logFile, "create", "-b", bundlePath, containerId)
		// 		})

		// 		AfterEach(func() {
		// 			Expect(os.RemoveAll(logFile)).To(Succeed())
		// 			Expect(os.RemoveAll(mountFile)).To(Succeed())
		// 		})

		// 		It("ignores it and logs that it did so", func() {
		// 			contents, err := ioutil.ReadFile(logFile)
		// 			Expect(err).ToNot(HaveOccurred())
		// 			Expect(string(contents)).To(ContainSubstring("mount is not a directory, ignoring"))
		// 			Expect(string(contents)).To(ContainSubstring(fmt.Sprintf(`"mount":"%s"`, strings.Replace(mountFile, `\`, `\\`, -1))))
		// 		})
		// 	})
		// })

		// Context("when the bundle config.json specifies a container memory limit", func() {
		// 	var memLimitMB = uint64(128)

		// 	BeforeEach(func() {
		// 		memLimitBytes := memLimitMB * 1024 * 1024
		// 		bundleSpec.Windows.Resources = &specs.WindowsResources{
		// 			Memory: &specs.WindowsMemoryResources{
		// 				Limit: &memLimitBytes,
		// 			},
		// 		}
		// 	})

		// 	JustBeforeEach(func() {
		// 		pid := getContainerState(containerId).Pid
		// 		err := copy(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "consume.exe"), consumeBin)
		// 		Expect(err).NotTo(HaveOccurred())
		// 	})

		// 	grabMemory := func(mem int, exitCode int) string {
		// 		cmd := exec.Command(wincBin, "exec", containerId, "c:\\consume.exe", strconv.Itoa(mem*1024*1024))
		// 		stdErr := new(bytes.Buffer)
		// 		session, err := gexec.Start(cmd, GinkgoWriter, io.MultiWriter(stdErr, GinkgoWriter))
		// 		Expect(err).ToNot(HaveOccurred())
		// 		Eventually(session, defaultTimeout*2).Should(gexec.Exit(exitCode))
		// 		return stdErr.String()
		// 	}

		// 	It("is not constrained by smaller memory limit", func() {
		// 		Expect(grabMemory(10, 0)).To(Equal(""))
		// 	})

		// 	It("is constrained by hitting the memory limit", func() {
		// 		Expect(grabMemory(int(memLimitMB), 2)).To(ContainSubstring("fatal error: out of memory"))
		// 	})
		// })
	})

	// Context("when the mount source does not exist", func() {
	// 	BeforeEach(func() {
	// 		mountDest := "C:\\mnt"
	// 		mountSource := "C:\\not\\a\\directory\\mountsource"

	// 		mount := specs.Mount{Destination: mountDest, Source: mountSource}
	// 		bundleSpec.Mounts = []specs.Mount{mount}
	// 	})

	// 	It("errors and does not create the container", func() {
	// 		cmd := exec.Command(wincBin, "create", "-b", bundlePath, containerId)
	// 		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	// 		Expect(err).ToNot(HaveOccurred())
	// 		Eventually(session).Should(gexec.Exit(1))

	// 		Expect(containerExists(containerId)).To(BeFalse())
	// 	})
	// })

	// Context("when provided a container id that already exists", func() {
	// 	JustBeforeEach(func() {
	// 		_, _, err := execute(exec.Command(wincBin, "create", "-b", bundlePath, containerId))
	// 		Expect(err).NotTo(HaveOccurred())
	// 	})

	// 	AfterEach(func() {
	// 		_, _, err := execute(exec.Command(wincBin, "delete", containerId))
	// 		Expect(err).ToNot(HaveOccurred())
	// 	})

	// 	It("errors", func() {
	// 		cmd := exec.Command(wincBin, "create", "-b", bundlePath, containerId)
	// 		stdErr := new(bytes.Buffer)
	// 		session, err := gexec.Start(cmd, GinkgoWriter, io.MultiWriter(stdErr, GinkgoWriter))
	// 		Expect(err).ToNot(HaveOccurred())

	// 		Eventually(session).Should(gexec.Exit(1))
	// 		expectedErrorMsg := fmt.Sprintf("container with id already exists: %s", containerId)
	// 		Expect(stdErr.String()).To(ContainSubstring(expectedErrorMsg))
	// 	})
	// })

	// Context("when the bundle directory name and container id do not match", func() {
	// 	It("errors and does not create the container", func() {
	// 		newContainerId := randomContainerId()
	// 		cmd := exec.Command(wincBin, "create", "-b", bundlePath, newContainerId)
	// 		stdErr := new(bytes.Buffer)
	// 		session, err := gexec.Start(cmd, GinkgoWriter, io.MultiWriter(stdErr, GinkgoWriter))
	// 		Expect(err).ToNot(HaveOccurred())

	// 		Eventually(session).Should(gexec.Exit(1))
	// 		expectedErrorMsg := fmt.Sprintf("container id does not match bundle directory name: %s", newContainerId)
	// 		Expect(stdErr.String()).To(ContainSubstring(expectedErrorMsg))

	// 		Expect(containerExists(newContainerId)).To(BeFalse())
	// 	})
	// })
})
