package hcs_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"testing"
)

var rootfsPath string

func TestHcs(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		var present bool
		rootfsPath, present = os.LookupEnv("WINC_TEST_ROOTFS")
		Expect(present).To(BeTrue(), "WINC_TEST_ROOTFS not set")
		logrus.SetOutput(ioutil.Discard)
	})

	RunSpecs(t, "Hcs Suite")
}
