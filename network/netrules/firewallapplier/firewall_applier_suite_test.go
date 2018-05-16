package firewallapplier_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestNetrules(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Firewall Applier Suite")
}
