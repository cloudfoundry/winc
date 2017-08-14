package sandbox_test

import (
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/winc/sandbox"
	"code.cloudfoundry.org/winc/sandbox/sandboxfakes"
	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sandbox", func() {
	const containerVolume = "containerVolume"

	var (
		storePath          string
		rootfs             string
		containerId        string
		hcsClient          *sandboxfakes.FakeHCSClient
		limiter            *sandboxfakes.FakeLimiter
		statser            *sandboxfakes.FakeStatser
		sandboxManager     *sandbox.Manager
		expectedDriverInfo hcsshim.DriverInfo
		rootfsParents      []byte
	)

	BeforeEach(func() {
		var err error
		rootfs, err = ioutil.TempDir("", "rootfs")
		Expect(err).ToNot(HaveOccurred())

		storePath, err = ioutil.TempDir("", "sandbox-store")
		Expect(err).ToNot(HaveOccurred())

		rand.Seed(time.Now().UnixNano())
		containerId = strconv.Itoa(rand.Int())

		hcsClient = &sandboxfakes.FakeHCSClient{}
		limiter = &sandboxfakes.FakeLimiter{}
		statser = &sandboxfakes.FakeStatser{}
		sandboxManager = sandbox.NewManager(hcsClient, limiter, statser, storePath, containerId)

		expectedDriverInfo = hcsshim.DriverInfo{
			HomeDir: storePath,
			Flavour: 1,
		}
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
			hcsClient.LayerExistsReturns(true, nil)
			logrus.SetOutput(ioutil.Discard)
		})

		It("unprepares, deactivates, and destroys the sandbox", func() {
			err := sandboxManager.Delete()
			Expect(err).ToNot(HaveOccurred())

			Expect(hcsClient.LayerExistsCallCount()).To(Equal(1))
			driverInfo, actualContainerId := hcsClient.LayerExistsArgsForCall(0)
			Expect(driverInfo).To(Equal(expectedDriverInfo))
			Expect(actualContainerId).To(Equal(containerId))

			Expect(hcsClient.DestroyLayerCallCount()).To(Equal(1))
			driverInfo, actualContainerId = hcsClient.DestroyLayerArgsForCall(0)
			Expect(driverInfo).To(Equal(expectedDriverInfo))
			Expect(actualContainerId).To(Equal(containerId))
		})

		Context("when checking if the layer exists fails", func() {
			var layerExistsError = errors.New("layer exists failed")

			BeforeEach(func() {
				hcsClient.LayerExistsReturns(false, layerExistsError)
			})

			It("errors", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(layerExistsError))
			})
		})

		Context("when the sandbox layer does not exist", func() {
			BeforeEach(func() {
				hcsClient.LayerExistsReturns(false, nil)
			})

			It("returns nil and does not try to delete the layer", func() {
				Expect(sandboxManager.Delete()).To(Succeed())
				Expect(hcsClient.LayerExistsCallCount()).To(Equal(1))
				Expect(hcsClient.DestroyLayerCallCount()).To(Equal(0))
			})
		})

		Context("when destroying the sandbox fails with a non-retryable error", func() {
			var destroyLayerError = errors.New("destroy sandbox failed (non-retryable)")

			BeforeEach(func() {
				hcsClient.DestroyLayerReturns(destroyLayerError)
				hcsClient.RetryableReturns(false)
			})

			It("errors immediately", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(destroyLayerError))

				Expect(hcsClient.DestroyLayerCallCount()).To(Equal(1))
				Expect(hcsClient.RetryableCallCount()).To(Equal(1))
				Expect(hcsClient.RetryableArgsForCall(0)).To(Equal(destroyLayerError))
			})
		})

		Context("when destroying the sandbox fails with a retryable error", func() {
			var destroyLayerError = errors.New("destroy sandbox failed (retryable)")

			BeforeEach(func() {
				hcsClient.DestroyLayerReturns(destroyLayerError)
				hcsClient.RetryableReturns(true)
			})

			It("tries to destroy the sandbox DESTROY_ATTEMPTS times before returning an error", func() {
				err := sandboxManager.Delete()
				Expect(err).To(Equal(destroyLayerError))

				Expect(hcsClient.DestroyLayerCallCount()).To(Equal(sandbox.DESTROY_ATTEMPTS))
				Expect(hcsClient.RetryableCallCount()).To(Equal(sandbox.DESTROY_ATTEMPTS))
				for i := 0; i < sandbox.DESTROY_ATTEMPTS; i++ {
					Expect(hcsClient.RetryableArgsForCall(i)).To(Equal(destroyLayerError))
				}
			})
		})

		Context("when destroying the sandbox fails with a retryable error and eventually succeeds", func() {
			var destroyLayerError = errors.New("destroy sandbox failed (retryable)")

			BeforeEach(func() {
				hcsClient.DestroyLayerReturnsOnCall(0, destroyLayerError)
				hcsClient.DestroyLayerReturnsOnCall(1, destroyLayerError)
				hcsClient.DestroyLayerReturnsOnCall(2, nil)
				hcsClient.RetryableReturns(true)
			})

			It("tries to destroy the sandbox three times", func() {
				Expect(sandboxManager.Delete()).To(Succeed())

				Expect(hcsClient.DestroyLayerCallCount()).To(Equal(3))
				Expect(hcsClient.RetryableCallCount()).To(Equal(2))
				Expect(hcsClient.RetryableArgsForCall(0)).To(Equal(destroyLayerError))
				Expect(hcsClient.RetryableArgsForCall(1)).To(Equal(destroyLayerError))
			})
		})
	})
})
