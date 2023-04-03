package container_test

import (
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/winc/runtime/container"
	"code.cloudfoundry.org/winc/runtime/container/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("CredentialSpec", func() {
	const containerId = "container-id"
	var (
		credentialSpecPath     string
		credentialSpecContents string

		hcsClient        *fakes.HCSClient
		logger           *logrus.Entry
		containerManager *container.Manager
	)

	BeforeEach(func() {
		var err error
		credentialSpecFile, err := os.CreateTemp("", "")
		Expect(err).ToNot(HaveOccurred())

		credentialSpecPath = credentialSpecFile.Name()

		credentialSpecContents = "credential-spec-contents"
		Expect(os.WriteFile(credentialSpecPath, []byte(credentialSpecContents), 0644)).To(Succeed())

		hcsClient = &fakes.HCSClient{}
		logger = (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "create")

		containerManager = container.New(logger, hcsClient, containerId)
	})

	It("loads the credential spec from the path", func() {
		actual, err := containerManager.CredentialSpec(credentialSpecPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).To(Equal(credentialSpecContents))
	})

	Context("when the path is invalid", func() {
		It("returns the error", func() {
			_, err := containerManager.CredentialSpec("/not/a/valid/path")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("The system cannot find the path specified"))
		})
	})

	Context("when the path is empty", func() {
		It("returns an empty string", func() {
			actual, err := containerManager.CredentialSpec("")
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(BeEmpty())
		})
	})
})
