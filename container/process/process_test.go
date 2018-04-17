package process_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/container/process"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ProcessClient", func() {
	var client *process.Client
	var fakeProcess *hcsfakes.Process
	var tempDir string

	BeforeEach(func() {
		fakeProcess = &hcsfakes.Process{}
		client = process.NewClient(fakeProcess)
		var err error
		tempDir, err = ioutil.TempDir("", "process")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Context("WritePIDFile", func() {
		BeforeEach(func() {
			fakeProcess.PidReturns(1034)
		})

		It("writes a pid to a file", func() {
			pidFile := filepath.Join(tempDir, "pidfile")
			Expect(client.WritePIDFile(pidFile)).To(Succeed())
			Expect(fakeProcess.PidCallCount()).To(Equal(1))
			Expect(pidFile).To(BeAnExistingFile())
			content, err := ioutil.ReadFile(pidFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("1034"))
		})

		It("doesn't write to an empty pid file", func() {
			pidFile := ""
			Expect(client.WritePIDFile(pidFile)).To(Succeed())
			Expect(fakeProcess.PidCallCount()).To(Equal(0))
			Expect(pidFile).NotTo(BeAnExistingFile())
		})
	})

	Context("AttachIO", func() {
		var (
			copiedStdin,
			copiedStdout,
			copiedStderr,
			processStdin,
			processStdout,
			processStderr *gbytes.Buffer
		)

		BeforeEach(func() {
			copiedStdin = gbytes.BufferWithBytes([]byte("something-on-stdin"))
			copiedStdout = gbytes.NewBuffer()
			copiedStderr = gbytes.NewBuffer()

			processStdin = gbytes.NewBuffer()
			processStdout = gbytes.BufferWithBytes([]byte("something-on-stdout"))
			processStderr = gbytes.BufferWithBytes([]byte("something-on-stderr"))

			fakeProcess.StdioReturns(processStdin, processStdout, processStderr, nil)
		})

		It("attaches process IO to stdin, stdout, and stderr", func() {
			// fakeProcess.WaitStub = func() error {
			// 	copiedStdout.Close()
			// 	return nil
			// }
			exitCode, err := client.AttachIO(copiedStdin, copiedStdout, copiedStderr)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))
			Eventually(processStdin).Should(gbytes.Say("something-on-stdin"))
			Eventually(copiedStdout).Should(gbytes.Say("something-on-stdout"))
			Eventually(copiedStderr).Should(gbytes.Say("something-on-stderr"))
		})

		Context("when getting the stdio streams fails", func() {
			BeforeEach(func() {
				fakeProcess.StdioReturns(nil, nil, nil, errors.New("some error"))
			})

			It("returns the error", func() {
				_, err := client.AttachIO(copiedStdin, copiedStdout, copiedStderr)
				Expect(err).To(MatchError("some error"))
			})
		})

		Context("when waiting on the process fails", func() {
			BeforeEach(func() {
				fakeProcess.WaitReturns(errors.New("some error"))
			})

			It("returns the error", func() {
				_, err := client.AttachIO(copiedStdin, copiedStdout, copiedStderr)
				Expect(err).To(MatchError("some error"))
			})
		})

		Context("when the process exits with a non-zero exit code", func() {
			BeforeEach(func() {
				fakeProcess.ExitCodeReturns(8, nil)
			})

			It("returns that exit code", func() {
				exitCode, err := client.AttachIO(copiedStdin, copiedStdout, copiedStderr)
				Expect(exitCode).To(Equal(8))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when getting the exit code fails", func() {
			BeforeEach(func() {
				fakeProcess.ExitCodeReturns(0, errors.New("some error"))
			})

			It("returns the error", func() {
				_, err := client.AttachIO(copiedStdin, copiedStdout, copiedStderr)
				Expect(err).To(MatchError("some error"))
			})
		})
	})
})
