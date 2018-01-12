package main_test

import (
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
	)

	BeforeEach(func() {
		var err error
		containerId = helpers.RandomContainerId()
		storePath, err = ioutil.TempDir("", "container-store")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		helpers.DeleteSandbox(storePath, containerId)
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Context("a disk limit is set", func() {
		BeforeEach(func() {
			createWithDiskLimit(storePath, rootfsPath, containerId, 300*1024*1024)
		})

		It("reports the image stats", func() {
			imageStats := getImageStats(storePath, containerId)
			Expect(imageStats.Disk.TotalBytesUsed).To(BeNumerically("~", 7*1024, 3*1024))
			Expect(imageStats.Disk.ExclusiveBytesUsed).To(Equal(uint64(0)))
		})

		Context("a large file is written", func() {
			var (
				mountPath string
				fileSize  uint64
			)

			BeforeEach(func() {
				var err error
				fileSize = 30 * 1024 * 1024
				mountPath, err = ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())

				mountSandboxVolume(storePath, containerId, mountPath)
				largeFilePath := filepath.Join(mountPath, "file.txt")
				Expect(exec.Command("fsutil", "file", "createnew", largeFilePath, strconv.FormatUint(fileSize, 10)).Run()).To(Succeed())
			})

			AfterEach(func() {
				unmountSandboxVolume(mountPath)
				Expect(os.RemoveAll(mountPath)).To(Succeed())
				helpers.DeleteSandbox(storePath, containerId)
			})

			It("includes the file in disk usage", func() {
				imageStats := getImageStats(storePath, containerId)
				Expect(imageStats.Disk.TotalBytesUsed).To(BeNumerically("~", fileSize+7*1024, 3*1024))
				Expect(imageStats.Disk.ExclusiveBytesUsed).To(BeNumerically("~", fileSize, 1024))
			})
		})
	})

	Context("no disk limit is set", func() {
		BeforeEach(func() {
			createWithDiskLimit(storePath, rootfsPath, containerId, 0)
		})

		It("returns usage as 0", func() {
			imageStats := getImageStats(storePath, containerId)
			Expect(imageStats.Disk.TotalBytesUsed).To(Equal(uint64(0)))
			Expect(imageStats.Disk.ExclusiveBytesUsed).To(Equal(uint64(0)))
		})
	})
})
