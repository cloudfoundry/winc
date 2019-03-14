package main_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	acl "github.com/hectane/go-acl"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/windows"
)

var _ = Describe("Events", func() {
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

			bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
			bundleSpec.Mounts = []specs.Mount{{Source: filepath.Dir(sleepBin), Destination: "C:\\tmp"}}
			Expect(acl.Apply(filepath.Dir(sleepBin), false, false, acl.GrantName(windows.GENERIC_ALL, "Everyone"))).To(Succeed())
			helpers.RunContainer(bundleSpec, bundlePath, containerId)
		})

		AfterEach(func() {
			failed = failed || CurrentGinkgoTestDescription().Failed
			helpers.DeleteContainer(containerId)
			helpers.DeleteVolume(containerId)
			Expect(os.RemoveAll(bundlePath)).To(Succeed())
		})

		Context("when the container has been created", func() {
			It("exits without error", func() {
				cmd := exec.Command(wincBin, "events", containerId)
				stdOut, stdErr, err := helpers.Execute(cmd)
				Expect(err).NotTo(HaveOccurred(), stdOut.String(), stdErr.String())
			})

			Context("when passed the --stats flag", func() {
				BeforeEach(func() {
					pid := helpers.GetContainerState(containerId).Pid
					helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "consume.exe"), consumeBin)
				})

				It("prints the container memory stats to stdout", func() {
					stats := getStats(containerId)
					Expect(stats.Data.Memory.Stats.TotalRss).To(BeNumerically(">", 0))

					memConsumedBytes := 200 * 1024 * 1024

					cmd := exec.Command(wincBin, "exec", containerId, "c:\\consume.exe", strconv.Itoa(memConsumedBytes), "10")
					stdOut, err := cmd.StdoutPipe()
					Expect(err).NotTo(HaveOccurred())

					Expect(cmd.Start()).To(Succeed())

					Eventually(func() string {
						out := make([]byte, 256, 256)
						n, _ := stdOut.Read(out)
						return strings.TrimSpace(string(out[:n]))
					}).Should(Equal(fmt.Sprintf("Allocated %d", memConsumedBytes)))

					statsAfter := getStats(containerId)
					goRuntimeOverhead := uint64(25 * 1024 * 1024)
					expectedMemConsumedBytes := stats.Data.Memory.Stats.TotalRss + uint64(memConsumedBytes) + goRuntimeOverhead
					threshold := 30 * 1024 * 1024
					Expect(statsAfter.Data.Memory.Stats.TotalRss).To(BeNumerically("~", expectedMemConsumedBytes, threshold))
					Expect(cmd.Wait()).To(Succeed())
				})

				It("prints the container CPU stats to stdout", func() {
					cpuUsageBefore := getStats(containerId).Data.CPUStats.CPUUsage.Usage
					Expect(cpuUsageBefore).To(BeNumerically(">", 0))

					args := []string{"powershell.exe", "-Command", "$result = 1; foreach ($number in 1..2147483647) {$result = $result * $number};"}
					stdOut, stdErr, err := helpers.ExecInContainer(containerId, args, true)
					Expect(err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())

					cpuUsageAfter := getStats(containerId).Data.CPUStats.CPUUsage.Usage
					Expect(cpuUsageAfter).To(BeNumerically(">", cpuUsageBefore))
				})

				It("prints the number of running processes stats to stdout", func() {
					// Wait for the number of processes to settle on 2019...ugh...
					time.Sleep(1 * time.Second)

					pidCountBefore := getStats(containerId).Data.Pids.Current
					Expect(pidCountBefore).To(BeNumerically(">", 0))

					var numberOfProcesses uint64 = 15
					var tolerance uint64 = 2
					var i uint64

					/*
					* There are many auxiliary processes that's run inside the container
					* by Windows which may spawn or die in between our measurements of
					* @pidCountBefore and @pidCountAfter. To compensate for this uncertainity,
					* we create a sufficiently large number of new processes and allow a
					* tolerance in our expection of (pidCountAfter - pidCountBefore).
					 */
					for i = 0; i < numberOfProcesses; i++ {
						signal := fmt.Sprintf("randomsignal%d", i)
						args := []string{"waitfor", signal, "/T", "9999"}
						stdOut, stdErr, err := helpers.ExecInContainer(containerId, args, true)
						Expect(err).ToNot(HaveOccurred(), stdOut.String(), stdErr.String())
					}

					pidCountAfter := getStats(containerId).Data.Pids.Current
					Expect(pidCountAfter).To(BeNumerically(">", pidCountBefore+numberOfProcesses-tolerance))
					Expect(pidCountAfter).To(BeNumerically("<", pidCountBefore+numberOfProcesses+tolerance))
				})
			})
		})
	})

	Context("given a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "events", "doesntexist")
			stdOut, stdErr, err := helpers.Execute(cmd)
			Expect(err).To(HaveOccurred(), stdOut.String(), stdErr.String())

			Expect(stdErr.String()).To(ContainSubstring("hcsshim::OpenComputeSystem doesntexist"))
			Expect(stdErr.String()).To(ContainSubstring("the specified identifier does not exist"))
		})
	})
})
