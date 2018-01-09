package netinterface_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestNetinterface(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NetInterface Suite")
}
