package main_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Flags", func() {
	var (
		args             []string
		err              error
		session          *gexec.Session
		expectedExitCode int
	)

	BeforeEach(func() {
		args = []string{}
		expectedExitCode = 0
	})

	JustBeforeEach(func() {
		wincCmd := exec.Command(wincBin, args...)
		session, err = gexec.Start(wincCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(session).Should(gexec.Exit(expectedExitCode))
	})

	Context("when passed a nonexistent flag", func() {
		BeforeEach(func() {
			args = []string{"--nonexistent"}
			expectedExitCode = 1
		})

		It("prints a message saying the flag does not exist", func() {
			Expect(session.Err).To(gbytes.Say("flag provided but not defined: -nonexistent"))
		})
	})

	Context("when passed no flags", func() {
		It("prints a help message", func() {
			Expect(session.Out).To(gbytes.Say("NAME:\n.*winc.exe - Open Container Initiative runtime for Windows"))
		})
	})

	Context("when passed '--help'", func() {
		BeforeEach(func() {
			args = []string{"--help"}
		})

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

	Context("when passed '--newuidmap'", func() {
		BeforeEach(func() {
			args = []string{"--newuidmap", "foo"}
		})

		It("accepts the flag and prints the --newuidmap flag usage", func() {
			Expect(session.Out).To(gbytes.Say("GLOBAL OPTIONS:(.|\n)*--newuidmap value"))
		})
	})

	Context("when passed '--newgidmap'", func() {
		BeforeEach(func() {
			args = []string{"--newgidmap", "foo"}
		})

		It("accepts the flag and prints the --newgidmap flag usage", func() {
			Expect(session.Out).To(gbytes.Say("GLOBAL OPTIONS:(.|\n)*--newgidmap value"))
		})
	})
})
