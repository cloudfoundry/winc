package firewallendpoint_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestFirewallEndpoint(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Firewall Endpoint Suite")
}
