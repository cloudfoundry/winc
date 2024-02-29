package runtime_test

import (
	"io"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

func TestRuntime(t *testing.T) {
	RegisterFailHandler(Fail)

	logrus.SetOutput(io.Discard)

	RunSpecs(t, "Runtime Suite")
}
