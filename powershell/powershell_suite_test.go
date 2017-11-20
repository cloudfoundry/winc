package powershell_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPowershell(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Powershell Suite")
}
