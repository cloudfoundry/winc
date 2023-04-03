package netsh_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestNetsh(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Netsh Suite")
}
