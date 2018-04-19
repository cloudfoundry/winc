package hcsprocess_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/container/hcsprocess"
	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Process", func() {
	var wrappedProcess *hcsprocess.Process
	var fakeProcess *hcsfakes.Process
	var tempDir string

	BeforeEach(func() {
		fakeProcess = &hcsfakes.Process{}
		wrappedProcess = hcsprocess.New(fakeProcess)
		var err error
		tempDir, err = ioutil.TempDir("", "process")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("WritePIDFile", func() {
		BeforeEach(func() {
			fakeProcess.PidReturns(1034)
		})

		It("writes a pid to a file", func() {
			pidFile := filepath.Join(tempDir, "pidfile")
			Expect(wrappedProcess.WritePIDFile(pidFile)).To(Succeed())
			Expect(fakeProcess.PidCallCount()).To(Equal(1))
			Expect(pidFile).To(BeAnExistingFile())
			content, err := ioutil.ReadFile(pidFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("1034"))
		})

		It("doesn't write to an empty pid file", func() {
			pidFile := ""
			Expect(wrappedProcess.WritePIDFile(pidFile)).To(Succeed())
			Expect(fakeProcess.PidCallCount()).To(Equal(0))
			Expect(pidFile).NotTo(BeAnExistingFile())
		})
	})

	Describe("AttachIO", func() {
		var (
			attachedStdin,
			attachedStdout,
			attachedStderr,
			processStdin,
			processStdout,
			processStderr *gbytes.Buffer
		)

		BeforeEach(func() {
			attachedStdin = gbytes.BufferWithBytes([]byte("something-on-stdin"))
			attachedStdout = gbytes.NewBuffer()
			attachedStderr = gbytes.NewBuffer()

			processStdin = gbytes.NewBuffer()
			processStdout = gbytes.BufferWithBytes([]byte("something-on-stdout"))
			processStderr = gbytes.BufferWithBytes([]byte("something-on-stderr"))

			fakeProcess.StdioReturns(processStdin, processStdout, processStderr, nil)
		})

		It("attaches process IO to stdin, stdout, and stderr", func() {
			exitCode, err := wrappedProcess.AttachIO(attachedStdin, attachedStdout, attachedStderr)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))
			Eventually(processStdin).Should(gbytes.Say("something-on-stdin"))
			Eventually(attachedStdout).Should(gbytes.Say("something-on-stdout"))
			Eventually(attachedStderr).Should(gbytes.Say("something-on-stderr"))
		})

		Context("when getting the stdio streams fails", func() {
			BeforeEach(func() {
				fakeProcess.StdioReturns(nil, nil, nil, errors.New("some error"))
			})

			It("returns the error", func() {
				_, err := wrappedProcess.AttachIO(attachedStdin, attachedStdout, attachedStderr)
				Expect(err).To(MatchError("some error"))
			})
		})

		Context("when waiting on the process fails", func() {
			BeforeEach(func() {
				fakeProcess.WaitReturns(errors.New("some error"))
			})

			It("returns the error", func() {
				_, err := wrappedProcess.AttachIO(attachedStdin, attachedStdout, attachedStderr)
				Expect(err).To(MatchError("some error"))
			})
		})

		Context("when the process exits with a non-zero exit code", func() {
			BeforeEach(func() {
				fakeProcess.ExitCodeReturns(8, nil)
			})

			It("returns that exit code", func() {
				exitCode, err := wrappedProcess.AttachIO(attachedStdin, attachedStdout, attachedStderr)
				Expect(exitCode).To(Equal(8))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when getting the exit code fails", func() {
			BeforeEach(func() {
				fakeProcess.ExitCodeReturns(0, errors.New("some error"))
			})

			It("returns the error", func() {
				_, err := wrappedProcess.AttachIO(attachedStdin, attachedStdout, attachedStderr)
				Expect(err).To(MatchError("some error"))
			})
		})

		Context("attached stdin is nil", func() {
			It("attaches stdout and stderr", func() {
				_, err := wrappedProcess.AttachIO(nil, attachedStdout, attachedStderr)
				Expect(err).NotTo(HaveOccurred())
				Expect(processStdin.Contents()).To(Equal([]byte{}))
				Eventually(attachedStdout).Should(gbytes.Say("something-on-stdout"))
				Eventually(attachedStderr).Should(gbytes.Say("something-on-stderr"))
			})
		})

		Context("attached stdout is nil", func() {
			It("attaches stdin and stderr", func() {
				_, err := wrappedProcess.AttachIO(attachedStdin, nil, attachedStderr)
				Expect(err).NotTo(HaveOccurred())
				Eventually(processStdin).Should(gbytes.Say("something-on-stdin"))
				Expect(attachedStdout.Contents()).To(Equal([]byte{}))
				Eventually(attachedStderr).Should(gbytes.Say("something-on-stderr"))
			})
		})

		Context("attached stdin is nil", func() {
			It("attaches stdout and stderr", func() {
				_, err := wrappedProcess.AttachIO(attachedStdin, attachedStdout, nil)
				Expect(err).NotTo(HaveOccurred())
				Eventually(processStdin).Should(gbytes.Say("something-on-stdin"))
				Eventually(attachedStdout).Should(gbytes.Say("something-on-stdout"))
				Expect(attachedStderr.Contents()).To(Equal([]byte{}))
			})
		})
	})

	Describe("SetInterrupt", func() {
		It("kills the process when interrupt is recieved", func() {
			signal := make(chan os.Signal, 1)
			wrappedProcess.SetInterrupt(signal)
			signal <- os.Interrupt
			Eventually(fakeProcess.KillCallCount).Should(Equal(1))
		})
	})
})
