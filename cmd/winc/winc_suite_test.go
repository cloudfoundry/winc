package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var wincBin string

func TestWinc(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeEach(func() {
		var err error
		wincBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		gexec.CleanupBuildArtifacts()
	})

	RunSpecs(t, "Winc Suite")
}
