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
		args    []string
		err     error
		session *gexec.Session
	)

	BeforeEach(func() {
		args = []string{"--help"}
	})

	JustBeforeEach(func() {
		wincCmd := exec.Command(wincBin, args...)
		session, err = gexec.Start(wincCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(session).Should(gexec.Exit(0))
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
})
