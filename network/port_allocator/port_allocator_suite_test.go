package port_allocator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPortAllocator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PortAllocator Suite")
}
