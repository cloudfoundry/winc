package image_test

import (
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/winc/image"
	"code.cloudfoundry.org/winc/image/imagefakes"
	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete", func() {
	const containerVolume = "containerVolume"

	var (
		storePath     string
		rootfs        string
		containerId   string
		layerManager  *imagefakes.FakeLayerManager
		limiter       *imagefakes.FakeLimiter
		statser       *imagefakes.FakeStatser
		imageManager  *image.Manager
		rootfsParents []byte
	)

	BeforeEach(func() {
		var err error
		rootfs, err = ioutil.TempDir("", "rootfs")
		Expect(err).ToNot(HaveOccurred())

		storePath, err = ioutil.TempDir("", "sandbox-store")
		Expect(err).ToNot(HaveOccurred())

		rand.Seed(time.Now().UnixNano())
		containerId = strconv.Itoa(rand.Int())

		layerManager = &imagefakes.FakeLayerManager{}
		limiter = &imagefakes.FakeLimiter{}
		statser = &imagefakes.FakeStatser{}
		imageManager = image.NewManager(layerManager, limiter, statser, containerId)

		rootfsParents = []byte(`["path1", "path2"]`)
	})

	JustBeforeEach(func() {
		Expect(ioutil.WriteFile(filepath.Join(rootfs, "layerchain.json"), rootfsParents, 0755)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
		Expect(os.RemoveAll(rootfs)).To(Succeed())
	})

	Context("Delete", func() {
		BeforeEach(func() {
			layerManager.LayerExistsReturns(true, nil)
			logrus.SetOutput(ioutil.Discard)
		})

		It("unprepares, deactivates, and destroys the sandbox", func() {
			err := imageManager.Delete()
			Expect(err).ToNot(HaveOccurred())

			Expect(layerManager.LayerExistsCallCount()).To(Equal(1))
			actualContainerId := layerManager.LayerExistsArgsForCall(0)
			Expect(actualContainerId).To(Equal(containerId))

			Expect(layerManager.RemoveLayerCallCount()).To(Equal(1))
			actualContainerId = layerManager.RemoveLayerArgsForCall(0)
			Expect(actualContainerId).To(Equal(containerId))
		})

		Context("when checking if the layer exists fails", func() {
			var layerExistsError = errors.New("layer exists failed")

			BeforeEach(func() {
				layerManager.LayerExistsReturns(false, layerExistsError)
			})

			It("errors", func() {
				err := imageManager.Delete()
				Expect(err).To(Equal(layerExistsError))
			})
		})

		Context("when the sandbox layer does not exist", func() {
			BeforeEach(func() {
				layerManager.LayerExistsReturns(false, nil)
			})

			It("returns nil and does not try to delete the layer", func() {
				Expect(imageManager.Delete()).To(Succeed())
				Expect(layerManager.LayerExistsCallCount()).To(Equal(1))
				Expect(layerManager.RemoveLayerCallCount()).To(Equal(0))
			})
		})

		Context("when destroying the sandbox fails with a non-retryable error", func() {
			var destroyLayerError = errors.New("destroy sandbox failed (non-retryable)")

			BeforeEach(func() {
				layerManager.RemoveLayerReturns(destroyLayerError)
				layerManager.RetryableReturns(false)
			})

			It("errors immediately", func() {
				err := imageManager.Delete()
				Expect(err).To(Equal(destroyLayerError))

				Expect(layerManager.RemoveLayerCallCount()).To(Equal(1))
				Expect(layerManager.RetryableCallCount()).To(Equal(1))
				Expect(layerManager.RetryableArgsForCall(0)).To(Equal(destroyLayerError))
			})
		})

		Context("when destroying the sandbox fails with a retryable error", func() {
			var destroyLayerError = errors.New("destroy sandbox failed (retryable)")

			BeforeEach(func() {
				layerManager.RemoveLayerReturns(destroyLayerError)
				layerManager.RetryableReturns(true)
			})

			It("tries to destroy the sandbox DESTROY_ATTEMPTS times before returning an error", func() {
				err := imageManager.Delete()
				Expect(err).To(Equal(destroyLayerError))

				Expect(layerManager.RemoveLayerCallCount()).To(Equal(image.DESTROY_ATTEMPTS))
				Expect(layerManager.RetryableCallCount()).To(Equal(image.DESTROY_ATTEMPTS))
				for i := 0; i < image.DESTROY_ATTEMPTS; i++ {
					Expect(layerManager.RetryableArgsForCall(i)).To(Equal(destroyLayerError))
				}
			})
		})

		Context("when destroying the sandbox fails with a retryable error and eventually succeeds", func() {
			var destroyLayerError = errors.New("destroy sandbox failed (retryable)")

			BeforeEach(func() {
				layerManager.RemoveLayerReturnsOnCall(0, destroyLayerError)
				layerManager.RemoveLayerReturnsOnCall(1, destroyLayerError)
				layerManager.RemoveLayerReturnsOnCall(2, nil)
				layerManager.RetryableReturns(true)
			})

			It("tries to destroy the sandbox three times", func() {
				Expect(imageManager.Delete()).To(Succeed())

				Expect(layerManager.RemoveLayerCallCount()).To(Equal(3))
				Expect(layerManager.RetryableCallCount()).To(Equal(2))
				Expect(layerManager.RetryableArgsForCall(0)).To(Equal(destroyLayerError))
				Expect(layerManager.RetryableArgsForCall(1)).To(Equal(destroyLayerError))
			})
		})
	})
})
