package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stats", func() {
	var (
		storePath   string
		containerId string
		args        []string
	)

	BeforeEach(func() {
		var err error
		containerId = randomContainerId()
		storePath, err = ioutil.TempDir("", "container-store")
		Expect(err).ToNot(HaveOccurred())

		args = []string{"--store", storePath, "create", "--disk-limit-size-bytes", strconv.Itoa(300 * 1024 * 1024), rootfsPath, containerId}
	})

	JustBeforeEach(func() {
		_, _, err := execute(wincImageBin, args...)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_, _, err := execute(wincImageBin, "--store", storePath, "delete", containerId)
		Expect(err).To(Succeed())
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	type DiskUsage struct {
		TotalBytesUsed     uint64 `json:"total_bytes_used"`
		ExclusiveBytesUsed uint64 `json:"exclusive_bytes_used"`
	}

	type ImageStats struct {
		Disk DiskUsage `json:"disk_usage"`
	}

	It("reports the image stats", func() {
		stdout, _, err := execute(wincImageBin, "--store", storePath, "stats", containerId)
		Expect(err).NotTo(HaveOccurred())
		var imageStats ImageStats
		Expect(json.Unmarshal(stdout.Bytes(), &imageStats)).To(Succeed())
		Expect(imageStats.Disk.TotalBytesUsed).To(BeNumerically("~", 5*1024, 1024))
		Expect(imageStats.Disk.ExclusiveBytesUsed).To(Equal(uint64(0)))
	})

	Context("no disk limit is set", func() {
		BeforeEach(func() {
			args = []string{"--store", storePath, "create", rootfsPath, containerId}
		})

		It("returns usage as 0", func() {
			stdout, _, err := execute(wincImageBin, "--store", storePath, "stats", containerId)
			Expect(err).NotTo(HaveOccurred())
			var imageStats ImageStats
			Expect(json.Unmarshal(stdout.Bytes(), &imageStats)).To(Succeed())
			Expect(imageStats.Disk.TotalBytesUsed).To(Equal(uint64(0)))
			Expect(imageStats.Disk.ExclusiveBytesUsed).To(Equal(uint64(0)))
		})
	})

	Context("a large file is written", func() {
		var (
			mountPath string
			fileSize  uint64
			err       error
		)

		BeforeEach(func() {
			fileSize = 30 * 1024 * 1024
		})

		JustBeforeEach(func() {
			mountPath, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			volumeGuid := getVolumeGuid(storePath, containerId)
			Expect(exec.Command("mountvol", mountPath, volumeGuid).Run()).To(Succeed())

			largeFilePath := filepath.Join(mountPath, "file.txt")
			Expect(exec.Command("fsutil", "file", "createnew", largeFilePath, strconv.FormatUint(fileSize, 10)).Run()).To(Succeed())
		})

		AfterEach(func() {
			Expect(exec.Command("mountvol", mountPath, "/D").Run()).To(Succeed())
			Expect(os.RemoveAll(mountPath)).To(Succeed())
		})

		It("includes the file in disk usage", func() {
			stdout, _, err := execute(wincImageBin, "--store", storePath, "stats", containerId)
			Expect(err).NotTo(HaveOccurred())
			var imageStats ImageStats
			Expect(json.Unmarshal(stdout.Bytes(), &imageStats)).To(Succeed())
			Expect(imageStats.Disk.TotalBytesUsed).To(BeNumerically("~", fileSize+5*1024, 1024))
			Expect(imageStats.Disk.ExclusiveBytesUsed).To(BeNumerically("~", fileSize, 1024))
		})
	})
})
