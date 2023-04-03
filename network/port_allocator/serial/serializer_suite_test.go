package serial_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSerializer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Serializer Suite")
}
