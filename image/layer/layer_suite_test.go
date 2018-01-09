package layer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var rootfsPath string

func TestHcs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Layer Suite")
}
