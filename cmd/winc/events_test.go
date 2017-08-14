package main_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/volume"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Events", func() {
	var (
		stdOut *bytes.Buffer
		stdErr *bytes.Buffer
	)

	BeforeEach(func() {
		stdOut = new(bytes.Buffer)
		stdErr = new(bytes.Buffer)
	})

	Context("given an existing container id", func() {
		var (
			containerId string
			cm          *container.Manager
			client      *hcs.Client
		)

		BeforeEach(func() {
			containerId = filepath.Base(bundlePath)

			client = &hcs.Client{}
			nm := networkManager(client)
			cm = container.NewManager(client, &volume.Mounter{}, nm, rootPath, bundlePath)

			bundleSpec := runtimeSpecGenerator(createSandbox(rootPath, rootfsPath, containerId), containerId)
			Expect(cm.Create(&bundleSpec)).To(Succeed())
		})

		AfterEach(func() {
			Expect(cm.Delete()).To(Succeed())
			Expect(execute(wincImageBin, "--store", rootPath, "delete", containerId)).To(Succeed())
		})

		Context("when the container has been created", func() {
			It("exits without error", func() {
				cmd := exec.Command(wincBin, "events", containerId)
				cmd.Stdout = stdOut
				Expect(cmd.Run()).To(Succeed())
			})

			Context("when passed the --stats flag", func() {
				type stats struct {
					Data struct {
						Memory struct {
							Stats struct {
								TotalRss uint64 `json:"total_rss"`
							} `json:"raw"`
						} `json:"memory"`
					} `json:"data"`
				}

				BeforeEach(func() {
					state, err := cm.State()
					Expect(err).ToNot(HaveOccurred())
					err = copy(filepath.Join("c:\\", "proc", strconv.Itoa(state.Pid), "root", "consume.exe"), consumeBin)
					Expect(err).NotTo(HaveOccurred())
				})

				It("prints the container memory stats to stdout", func() {
					cmd := exec.Command(wincBin, "events", "--stats", containerId)
					cmd.Stdout = stdOut
					Expect(cmd.Run()).To(Succeed())

					var statsBefore stats
					Expect(json.Unmarshal(stdOut.Bytes(), &statsBefore)).To(Succeed())
					Expect(statsBefore.Data.Memory.Stats.TotalRss).To(BeNumerically(">", 0))

					memConsumedBytes := 100 * 1024 * 1024

					err := exec.Command(wincBin, "exec", "-d", containerId, "c:\\consume.exe", strconv.Itoa(memConsumedBytes), "10").Run()
					Expect(err).ToNot(HaveOccurred())

					stdOut = new(bytes.Buffer)
					cmd = exec.Command(wincBin, "events", "--stats", containerId)
					cmd.Stdout = stdOut
					Expect(cmd.Run()).To(Succeed())

					expectedMemConsumedBytes := statsBefore.Data.Memory.Stats.TotalRss + uint64(memConsumedBytes)
					threshold := 15 * 1024 * 1024

					var statsAfter stats
					Expect(json.Unmarshal(stdOut.Bytes(), &statsAfter)).To(Succeed())
					Expect(statsAfter.Data.Memory.Stats.TotalRss).To(BeNumerically("~", expectedMemConsumedBytes, threshold))
				})
			})
		})
	})

	Context("given a nonexistent container id", func() {
		It("errors", func() {
			cmd := exec.Command(wincBin, "events", "doesntexist")
			session, err := gexec.Start(cmd, stdOut, stdErr)
			Expect(err).ToNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			expectedError := &hcs.NotFoundError{Id: "doesntexist"}
			Expect(stdErr.String()).To(ContainSubstring(expectedError.Error()))
		})
	})
})
