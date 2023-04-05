package container_test

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"code.cloudfoundry.org/winc/runtime/config"
	"code.cloudfoundry.org/winc/runtime/container"
	"code.cloudfoundry.org/winc/runtime/container/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Spec", func() {
	const (
		containerVolume = "containervolume"
		hostName        = "some-hostname"
	)

	var (
		containerId      string
		bundlePath       string
		layerFolders     []string
		hcsClient        *fakes.HCSClient
		containerManager *container.Manager
		spec             *specs.Spec
		logger           *logrus.Entry
	)

	BeforeEach(func() {
		var err error
		bundlePath, err = ioutil.TempDir("", "bundlePath")
		Expect(err).ToNot(HaveOccurred())
		containerId = filepath.Base(bundlePath)

		layerFolders = []string{
			"some-layer",
			"some-other-layer",
			"some-rootfs",
		}

		spec = &specs.Spec{
			Version: specs.Version,
			Process: &specs.Process{
				Args: []string{"cmd.exe"},
				Cwd:  "C:\\",
			},
			Root: &specs.Root{
				Path: containerVolume,
			},
			Windows: &specs.Windows{
				LayerFolders: layerFolders,
			},
			Hostname: hostName,
		}
		writeSpec(bundlePath, spec)

		hcsClient = &fakes.HCSClient{}
		logger = (&logrus.Logger{
			Out: ioutil.Discard,
		}).WithField("test", "create")

		containerManager = container.New(logger, hcsClient, containerId)
	})

	It("loads and validates the spec from the bundle path", func() {
		returnedSpec, err := containerManager.Spec(bundlePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(*returnedSpec).To(Equal(*spec))
	})

	Context("the bundle fails validation", func() {
		BeforeEach(func() {
			spec.Root.Path = ""
			writeSpec(bundlePath, spec)
		})

		It("returns an error", func() {
			_, err := containerManager.Spec(bundlePath)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&config.BundleConfigValidationError{}))
		})
	})

	Context("the process config fails validation", func() {
		BeforeEach(func() {
			spec.Process.Cwd = "./some/dir"
			writeSpec(bundlePath, spec)
		})

		It("returns an error", func() {
			_, err := containerManager.Spec(bundlePath)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&config.ProcessConfigValidationError{}))
		})
	})

	Context("the container id doesn't match the bundle path", func() {
		BeforeEach(func() {
			containerManager = container.New(logger, hcsClient, "a-different-id")
		})

		It("returns an error", func() {
			_, err := containerManager.Spec(bundlePath)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&container.InvalidIdError{}))
		})
	})
})

func writeSpec(bundlePath string, spec *specs.Spec) {
	contents, err := json.Marshal(spec)
	Expect(err).NotTo(HaveOccurred())
	Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), contents, 0644)).To(Succeed())
}
