package mount_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMount(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mount Suite")
}
