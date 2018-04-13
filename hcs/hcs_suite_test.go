package hcs_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHcs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hcs Suite")
}
