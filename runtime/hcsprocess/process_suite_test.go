package hcsprocess_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestProcess(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HCSProcess Suite")
}
