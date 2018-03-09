package main_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	acl "github.com/hectane/go-acl"
	ps "github.com/mitchellh/go-ps"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/windows"
)

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

		bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
	})

	AfterEach(func() {
		helpers.DeleteVolume(containerId)
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Context("when provided valid arguments", func() {
		AfterEach(func() {
			helpers.DeleteContainer(containerId)
		})

		It("creates and starts a container", func() {
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
			Expect(helpers.ContainerExists(containerId)).To(BeTrue())
			Expect(ps.FindProcess(helpers.GetContainerState(containerId).Pid)).ToNot(BeNil())
		})

		It("mounts the sandbox.vhdx at C:\\proc\\<pid>\\root", func() {
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)

			pid := helpers.GetContainerState(containerId).Pid
			Expect(ioutil.WriteFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "test.txt"), []byte("contents"), 0644)).To(Succeed())

			stdOut, _, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "type", "test.txt"}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("contents"))
		})

		Context("when the bundle path is not provided", func() {
			It("uses the current directory as the bundle path", func() {
				helpers.GenerateBundle(bundleSpec, bundlePath)
				createCmd := exec.Command(wincBin, "create", containerId)
				createCmd.Dir = bundlePath
				stdOut, stdErr, err := helpers.Execute(createCmd)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				Expect(helpers.ContainerExists(containerId)).To(BeTrue())
			})
		})

		Context("when the bundle path ends with a \\", func() {
			It("creates a container sucessfully", func() {
				helpers.CreateContainer(bundleSpec, bundlePath+"\\", containerId)
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
				helpers.GenerateBundle(bundleSpec, bundlePath)
				stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, "--pid-file", pidFile, containerId))
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

				containerPid := helpers.GetContainerState(containerId).Pid

				pidBytes, err := ioutil.ReadFile(pidFile)
				Expect(err).ToNot(HaveOccurred())
				pid, err := strconv.ParseInt(string(pidBytes), 10, 64)
				Expect(err).ToNot(HaveOccurred())
				Expect(int(pid)).To(Equal(containerPid))
			})
		})

		Context("when the '--no-new-keyring' flag is provided", func() {
			It("ignores it and creates and starts a container", func() {
				helpers.GenerateBundle(bundleSpec, bundlePath)
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
				helpers.GenerateBundle(bundleSpec, bundlePath)
				_, _, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, "--no-new-keyring", containerId))
				Expect(err).NotTo(HaveOccurred())

				stdOut, _, err := helpers.ExecInContainer(containerId, []string{"hostname"}, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.TrimSpace(stdOut.String())).To(Equal("some-random-hostname"))
			})
		})

		Context("when the bundle config.json specifies bind mounts", func() {
			var (
				mountSource string
				mountDest   string
			)

			BeforeEach(func() {
				var err error
				mountSource, err = ioutil.TempDir("", "mountsource")
				Expect(err).ToNot(HaveOccurred())
				Expect(ioutil.WriteFile(filepath.Join(mountSource, "sentinel"), []byte("hello"), 0644)).To(Succeed())
				Expect(acl.Apply(mountSource, false, false, acl.GrantName(windows.GENERIC_ALL, "Everyone"))).To(Succeed())

				mountDest = "C:\\mountdest"

				mount := specs.Mount{Destination: mountDest, Source: mountSource}
				bundleSpec.Mounts = []specs.Mount{mount}
			})

			AfterEach(func() {
				helpers.DeleteContainer(containerId)
				Expect(os.RemoveAll(mountSource)).To(Succeed())
			})

			It("creates a container with the specified directories as mounts", func() {
				helpers.CreateContainer(bundleSpec, bundlePath, containerId)
				stdOut, _, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "type", filepath.Join(mountDest, "sentinel")}, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(stdOut.String()).To(ContainSubstring("hello"))
			})

			Context("no mount options are specified", func() {
				It("the mounted directories are read only", func() {
					helpers.CreateContainer(bundleSpec, bundlePath, containerId)
					_, stdErr, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "echo hello > " + filepath.Join(mountDest, "sentinel2")}, false)
					Expect(err).To(HaveOccurred())
					Expect(stdErr.String()).To(ContainSubstring("Access is denied"))
				})
			})

			Context("the read-only mount option is specified", func() {
				BeforeEach(func() {
					bundleSpec.Mounts[0].Options = []string{"bind", "ro"}
				})

				It("the mounted directories are read only", func() {
					helpers.CreateContainer(bundleSpec, bundlePath, containerId)
					_, stdErr, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "echo hello > " + filepath.Join(mountDest, "sentinel2")}, false)
					Expect(err).To(HaveOccurred())
					Expect(stdErr.String()).To(ContainSubstring("Access is denied"))
				})
			})

			Context("the read/write mount option is specified", func() {
				BeforeEach(func() {
					bundleSpec.Mounts[0].Options = []string{"bind", "rw"}
				})

				It("the mounted directories can be written to", func() {
					helpers.CreateContainer(bundleSpec, bundlePath, containerId)
					_, _, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "echo hello2 > " + filepath.Join(mountDest, "sentinel2")}, false)
					Expect(err).ToNot(HaveOccurred())

					stdOut, _, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "type", filepath.Join(mountDest, "sentinel2")}, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(stdOut.String()).To(ContainSubstring("hello2"))
				})
			})

			Context("both read/write and read-only are specified", func() {
				BeforeEach(func() {
					bundleSpec.Mounts[0].Options = []string{"bind", "rw", "ro"}
				})

				It("errors", func() {
					helpers.GenerateBundle(bundleSpec, bundlePath)
					_, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, containerId))
					Expect(err).To(HaveOccurred())
					Expect(stdErr.String()).To(ContainSubstring(fmt.Sprintf("invalid mount options for container %s: [bind rw ro]", containerId)))
				})
			})

			FContext("the source of the bind mount is a symlink", func() {
				var symlinkDir string

				BeforeEach(func() {
					var err error
					symlinkDir, err = ioutil.TempDir("", "symlinkdir")
					Expect(err).ToNot(HaveOccurred())
					symlink := filepath.Join(symlinkDir, "link-dir")
					Expect(createSymlinkToDir(mountSource, symlink)).To(Succeed())

					bundleSpec.Mounts = []specs.Mount{{Destination: mountDest, Source: symlink}}
				})

				AfterEach(func() {
					Expect(os.RemoveAll(symlinkDir)).To(Succeed())
				})

				It("creates a container with the specified directories as mounts", func() {
					helpers.CreateContainer(bundleSpec, bundlePath, containerId)
					stdOut, _, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "type", filepath.Join(mountDest, "sentinel")}, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(stdOut.String()).To(ContainSubstring("hello"))
				})

				Context("the read-only mount option is specified", func() {
					BeforeEach(func() {
						bundleSpec.Mounts[0].Options = []string{"bind", "ro"}
					})

					It("the mounted directories are read only", func() {
						helpers.CreateContainer(bundleSpec, bundlePath, containerId)
						_, stdErr, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "echo hello > " + filepath.Join(mountDest, "sentinel2")}, false)
						Expect(err).To(HaveOccurred())
						Expect(stdErr.String()).To(ContainSubstring("Access is denied"))
					})
				})

				Context("the read/write mount option is specified", func() {
					BeforeEach(func() {
						bundleSpec.Mounts[0].Options = []string{"bind", "rw"}
					})

					It("the mounted directories can be written to", func() {
						helpers.CreateContainer(bundleSpec, bundlePath, containerId)
						_, _, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "echo hello2 > " + filepath.Join(mountDest, "sentinel2")}, false)
						Expect(err).ToNot(HaveOccurred())

						stdOut, _, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "type", filepath.Join(mountDest, "sentinel2")}, false)
						Expect(err).NotTo(HaveOccurred())
						Expect(stdOut.String()).To(ContainSubstring("hello2"))
					})
				})
			})

			Context("when the destination is /tmp/", func() {
				BeforeEach(func() {
					mountDest = "/tmp/mountdest"

					mount := specs.Mount{Destination: mountDest, Source: mountSource}
					bundleSpec.Mounts = []specs.Mount{mount}
				})

				It("mounts the specified directories", func() {
					helpers.CreateContainer(bundleSpec, bundlePath, containerId)

					stdOut, _, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "type", filepath.Join(mountDest, "sentinel")}, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(stdOut.String()).To(ContainSubstring("hello"))
				})

				Context("when calling the mounted executable", func() {
					BeforeEach(func() {
						helpers.CreateContainer(bundleSpec, bundlePath, containerId)

						helpers.CopyFile(filepath.Join(mountSource, "cmd.exe"), "C:\\Windows\\System32\\cmd.exe")
					})
					Context("when using the windows path", func() {
						It("mounts the specified directories", func() {
							stdOut, _, err := helpers.ExecInContainer(containerId, []string{filepath.Join(mountDest, "cmd"), "/C", "type", filepath.Join(mountDest, "sentinel")}, false)
							Expect(err).NotTo(HaveOccurred())
							Expect(stdOut.String()).To(ContainSubstring("hello"))
						})
					})
					Context("when using the unix path", func() {
						It("mounts the specified directories", func() {
							stdOut, _, err := helpers.ExecInContainer(containerId, []string{mountDest + "/cmd", "/C", "type", filepath.Join(mountDest, "sentinel")}, false)
							Expect(err).NotTo(HaveOccurred())
							Expect(stdOut.String()).To(ContainSubstring("hello"))
						})
					})
				})
			})

			Context("when a file is supplied as a mount", func() {
				var (
					logFile   string
					mountFile string
				)

				BeforeEach(func() {
					l, err := ioutil.TempFile("", "winc.log")
					Expect(err).ToNot(HaveOccurred())
					Expect(l.Close()).To(Succeed())
					logFile = l.Name()

					m, err := ioutil.TempFile("", "mountfile")
					Expect(err).ToNot(HaveOccurred())
					Expect(m.Close()).To(Succeed())
					mountFile = m.Name()

					bundleSpec.Mounts = append(bundleSpec.Mounts, specs.Mount{
						Source:      mountFile,
						Destination: "C:\\foobar",
					})
				})

				AfterEach(func() {
					Expect(os.RemoveAll(logFile)).To(Succeed())
					Expect(os.RemoveAll(mountFile)).To(Succeed())
				})

				It("ignores it and logs that it did so", func() {
					helpers.GenerateBundle(bundleSpec, bundlePath)
					stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "--debug", "--log", logFile, "create", "-b", bundlePath, containerId))
					Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())

					contents, err := ioutil.ReadFile(logFile)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(contents)).To(ContainSubstring("mount is not a directory, ignoring"))
					Expect(string(contents)).To(ContainSubstring(fmt.Sprintf(`"mount":"%s"`, strings.Replace(mountFile, `\`, `\\`, -1))))
				})
			})
		})

		Context("when the bundle config.json specifies a container memory limit", func() {
			var memLimitMB = uint64(128)

			BeforeEach(func() {
				memLimitBytes := memLimitMB * 1024 * 1024
				bundleSpec.Windows.Resources = &specs.WindowsResources{
					Memory: &specs.WindowsMemoryResources{
						Limit: &memLimitBytes,
					},
				}
			})

			grabMemory := func(mem int, exitCode int) string {
				cmd := exec.Command(wincBin, "exec", containerId, "c:\\consume.exe", strconv.Itoa(mem*1024*1024))
				stdErr := new(bytes.Buffer)
				session, err := gexec.Start(cmd, GinkgoWriter, io.MultiWriter(stdErr, GinkgoWriter))
				Expect(err).ToNot(HaveOccurred())
				Eventually(session, defaultTimeout*2).Should(gexec.Exit(exitCode))
				return stdErr.String()
			}

			It("is not constrained by smaller memory limit", func() {
				helpers.CreateContainer(bundleSpec, bundlePath, containerId)

				pid := helpers.GetContainerState(containerId).Pid
				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "consume.exe"), consumeBin)

				Expect(grabMemory(10, 0)).To(Equal(""))
			})

			It("is constrained by hitting the memory limit", func() {
				helpers.CreateContainer(bundleSpec, bundlePath, containerId)

				pid := helpers.GetContainerState(containerId).Pid
				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "consume.exe"), consumeBin)

				Expect(grabMemory(int(memLimitMB), 2)).To(ContainSubstring("fatal error: out of memory"))
			})
		})
	})

	Context("when the mount source does not exist", func() {
		BeforeEach(func() {
			mountDest := "C:\\mnt"
			mountSource := "C:\\not\\a\\directory\\mountsource"

			mount := specs.Mount{Destination: mountDest, Source: mountSource}
			bundleSpec.Mounts = []specs.Mount{mount}
		})

		It("errors and does not create the container", func() {
			helpers.GenerateBundle(bundleSpec, bundlePath)
			stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, containerId))
			Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())
			Expect(stdErr.String()).To(ContainSubstring(`CreateFile C:\not\a\directory\mountsource: The system cannot find the path specified`))

			Expect(helpers.ContainerExists(containerId)).To(BeFalse())
		})
	})

	Context("when provided a container id that already exists", func() {
		BeforeEach(func() {
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
		})

		AfterEach(func() {
			helpers.DeleteContainer(containerId)
		})

		It("errors", func() {
			stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, containerId))
			Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())
			expectedErrorMsg := fmt.Sprintf("container with id already exists: %s", containerId)
			Expect(stdErr.String()).To(ContainSubstring(expectedErrorMsg))
		})
	})

	Context("when the bundle directory name and container id do not match", func() {
		It("errors and does not create the container", func() {
			newContainerId := helpers.RandomContainerId()

			helpers.GenerateBundle(bundleSpec, bundlePath)
			stdOut, stdErr, err := helpers.Execute(exec.Command(wincBin, "create", "-b", bundlePath, newContainerId))
			Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())

			expectedErrorMsg := fmt.Sprintf("container id does not match bundle directory name: %s", newContainerId)
			Expect(stdErr.String()).To(ContainSubstring(expectedErrorMsg))

			Expect(helpers.ContainerExists(newContainerId)).To(BeFalse())
		})
	})
})

func createSymlinkToDir(oldname, newname string) error {
	// CreateSymbolicLink is not supported before Windows Vista
	if syscall.LoadCreateSymbolicLink() != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: syscall.EWINDOWS}
	}

	n, err := syscall.UTF16PtrFromString(newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
	}
	o, err := syscall.UTF16PtrFromString(oldname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
	}

	var flags uint32
	flags |= syscall.SYMBOLIC_LINK_FLAG_DIRECTORY
	err = syscall.CreateSymbolicLink(n, o, flags)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
	}
	return nil
}
