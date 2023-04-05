package mount_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMount(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mount Suite")
}
