package powershell_test

import (
	"code.cloudfoundry.org/winc/powershell"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Powershell", func() {

	var (
		ps *powershell.Powershell
	)

	BeforeEach(func() {
		ps = powershell.NewPowershell()
	})

	Describe("Run", func() {
		It("runs a powershell command", func() {
			output, err := ps.Run("write-host hello")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("hello"))
		})

		Context("when the command fails", func() {
			It("returns an error including the output", func() {
				errorMsg := `The term 'some-bad-command' is not recognized as the name of a cmdlet, function, script file`
				output, err := ps.Run("some-bad-command")
				Expect(err).To(MatchError(ContainSubstring(errorMsg)))
				Expect(output).To(ContainSubstring(errorMsg))
			})
		})
	})
})
