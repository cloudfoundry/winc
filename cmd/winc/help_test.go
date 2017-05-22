package main_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Help", func() {
	var (
		args             []string
		err              error
		session          *gexec.Session
		expectedExitCode int
	)

	BeforeEach(func() {
		args = []string{"--help"}
		expectedExitCode = 0
	})

	JustBeforeEach(func() {
		wincCmd := exec.Command(wincBin, args...)
		session, err = gexec.Start(wincCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(session).Should(gexec.Exit(expectedExitCode))
	})

	Context("when passed '--help'", func() {
		It("prints a help message", func() {
			Expect(session.Out).To(gbytes.Say("NAME:\n.*winc.exe - Open Container Initiative runtime for Windows"))
		})
	})

	Context("when passed '-h'", func() {
		BeforeEach(func() {
			args = []string{"-h"}
		})

		It("prints a help message", func() {
			Expect(session.Out).To(gbytes.Say("NAME:\n.*winc.exe - Open Container Initiative runtime for Windows"))
		})
	})

	Context("when passed a nonexistent command", func() {
		BeforeEach(func() {
			args = append([]string{"doesntexist"}, args...)
			expectedExitCode = 3
		})

		It("errors and prints the error message to stderr", func() {
			Expect(session.Err).To(gbytes.Say("No help topic for 'doesntexist'"))
		})
	})

	Context("create", func() {
		BeforeEach(func() {
			args = append([]string{"create"}, args...)
		})

		It("prints the create help message", func() {
			Expect(session.Out).To(gbytes.Say("NAME:\n.*winc.exe create - create a container"))
		})
	})

	Context("delete", func() {
		BeforeEach(func() {
			args = append([]string{"delete"}, args...)
		})

		It("prints the delete help message", func() {
			Expect(session.Out).To(gbytes.Say("NAME:\n.*winc.exe delete - delete a container and the resources it holds"))
		})
	})

	Context("state", func() {
		BeforeEach(func() {
			args = append([]string{"state"}, args...)
		})

		It("prints the state help message", func() {
			Expect(session.Out).To(gbytes.Say("NAME:\n.*winc.exe state - output the state of a container"))
		})
	})

	Context("exec", func() {
		BeforeEach(func() {
			args = append([]string{"exec"}, args...)
		})

		It("prints the exec help message", func() {
			Expect(session.Out).To(gbytes.Say("NAME:\n.*winc.exe exec - execute new process inside a container"))
		})
	})
})
