package main_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Events", func() {
	Context("given an existing container id", func() {
		var containerId string

		BeforeEach(func() {
			containerId = filepath.Base(bundlePath)

			bundleSpec := runtimeSpecGenerator(createSandbox(rootPath, rootfsPath, containerId))
			config, err := json.Marshal(&bundleSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0666)).To(Succeed())
			_, _, err = execute(exec.Command(wincBin, "create", "-b", bundlePath, containerId))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, _, err := execute(exec.Command(wincBin, "delete", containerId))
			Expect(err).NotTo(HaveOccurred())
			_, _, err = execute(exec.Command(wincImageBin, "--store", rootPath, "delete", containerId))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the container has been created", func() {
			It("exits without error", func() {
				cmd := exec.Command(wincBin, "events", containerId)
				_, _, err := execute(cmd)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when passed the --stats flag", func() {
				BeforeEach(func() {
					pid := getContainerState(containerId).Pid
					err := copy(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "consume.exe"), consumeBin)
					Expect(err).NotTo(HaveOccurred())
				})

				It("prints the container memory stats to stdout", func() {
					stats := getStats(containerId)
					Expect(stats.Data.Memory.Stats.TotalRss).To(BeNumerically(">", 0))

					memConsumedBytes := 200 * 1024 * 1024

					_, _, err := execute(exec.Command(wincBin, "exec", "-d", containerId, "c:\\consume.exe", strconv.Itoa(memConsumedBytes), "10"))
					Expect(err).ToNot(HaveOccurred())

					statsAfter := getStats(containerId)
					expectedMemConsumedBytes := stats.Data.Memory.Stats.TotalRss + uint64(memConsumedBytes)
					threshold := 30 * 1024 * 1024
					Expect(statsAfter.Data.Memory.Stats.TotalRss).To(BeNumerically("~", expectedMemConsumedBytes, threshold))
				})

				It("prints the container CPU stats to stdout", func() {
					cpuUsageBefore := getStats(containerId).Data.CPUStats.CPUUsage.Usage
					Expect(cpuUsageBefore).To(BeNumerically(">", 0))

					_, _, err := execute(exec.Command(wincBin, "exec", "-d", containerId, "powershell.exe", "-Command", "$result = 1; foreach ($number in 1..2147483647) {$result = $result * $number};"))
					Expect(err).ToNot(HaveOccurred())

					cpuUsageAfter := getStats(containerId).Data.CPUStats.CPUUsage.Usage
					Expect(cpuUsageAfter).To(BeNumerically(">", cpuUsageBefore))
				})
			})
		})
	})

	Context("given a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "events", "doesntexist")
			stdErr := new(bytes.Buffer)
			session, err := gexec.Start(cmd, GinkgoWriter, io.MultiWriter(GinkgoWriter, stdErr))
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(stdErr.String()).To(ContainSubstring("container not found: doesntexist"))
		})
	})
})

type wincStats struct {
	Data struct {
		CPUStats struct {
			CPUUsage struct {
				Usage  uint64 `json:"total"`
				System uint64 `json:"kernel"`
				User   uint64 `json:"user"`
			} `json:"usage"`
		} `json:"cpu"`
		Memory struct {
			Stats struct {
				TotalRss uint64 `json:"total_rss"`
			} `json:"raw"`
		} `json:"memory"`
	} `json:"data"`
}

func getStats(containerId string) wincStats {
	var stats wincStats
	stdOut, _, err := execute(exec.Command(wincBin, "events", "--stats", containerId))
	Expect(err).To(Succeed())
	Expect(json.Unmarshal(stdOut.Bytes(), &stats)).To(Succeed())
	return stats
}
