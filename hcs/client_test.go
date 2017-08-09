package hcs_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/winc/hcs"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	var (
		depot         string
		containerId   string
		driverInfo    hcsshim.DriverInfo
		sandboxLayers []string
		client        hcs.Client
	)

	BeforeEach(func() {
		var err error
		depot, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		driverInfo = hcsshim.DriverInfo{
			Flavour: 1,
			HomeDir: depot,
		}
		rand.Seed(time.Now().UnixNano())
		containerId = strconv.Itoa(rand.Int())

		parentLayerChain, err := ioutil.ReadFile(filepath.Join(rootfsPath, "layerchain.json"))
		Expect(err).NotTo(HaveOccurred())
		parentLayers := []string{}
		Expect(json.Unmarshal(parentLayerChain, &parentLayers)).To(Succeed())

		sandboxLayers = append([]string{rootfsPath}, parentLayers...)

		client = hcs.Client{}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(depot)).To(Succeed())
	})

	Describe("CreateLayer", func() {
		AfterEach(func() {
			Expect(client.DestroyLayer(driverInfo, containerId)).To(Succeed())
		})

		It("creates the layer", func() {
			volumeGuid, err := client.CreateLayer(driverInfo, containerId, rootfsPath, sandboxLayers)
			Expect(err).ToNot(HaveOccurred())

			expectedVolumeGuid, err := hcsshim.GetLayerMountPath(driverInfo, containerId)
			Expect(err).ToNot(HaveOccurred())
			Expect(volumeGuid).ToNot(BeEmpty())
			Expect(volumeGuid).To(Equal(expectedVolumeGuid))
		})

		Context("when the layer has been created but not activated", func() {
			BeforeEach(func() {
				Expect(hcsshim.CreateSandboxLayer(driverInfo, containerId, rootfsPath, sandboxLayers)).To(Succeed())
			})

			It("continues and creates the layer", func() {
				volumeGuid, err := client.CreateLayer(driverInfo, containerId, rootfsPath, sandboxLayers)
				Expect(err).ToNot(HaveOccurred())

				expectedVolumeGuid, err := hcsshim.GetLayerMountPath(driverInfo, containerId)
				Expect(err).ToNot(HaveOccurred())
				Expect(volumeGuid).ToNot(BeEmpty())
				Expect(volumeGuid).To(Equal(expectedVolumeGuid))
			})
		})

		Context("when the layer has been created and activated but not prepared", func() {
			BeforeEach(func() {
				Expect(hcsshim.CreateSandboxLayer(driverInfo, containerId, rootfsPath, sandboxLayers)).To(Succeed())
				Expect(hcsshim.ActivateLayer(driverInfo, containerId)).To(Succeed())
			})

			It("continues and creates the layer", func() {
				volumeGuid, err := client.CreateLayer(driverInfo, containerId, rootfsPath, sandboxLayers)
				Expect(err).ToNot(HaveOccurred())

				expectedVolumeGuid, err := hcsshim.GetLayerMountPath(driverInfo, containerId)
				Expect(err).ToNot(HaveOccurred())
				Expect(volumeGuid).ToNot(BeEmpty())
				Expect(volumeGuid).To(Equal(expectedVolumeGuid))
			})
		})

		Context("when the layer has been created, activated, and prepared", func() {
			BeforeEach(func() {
				Expect(hcsshim.CreateSandboxLayer(driverInfo, containerId, rootfsPath, sandboxLayers)).To(Succeed())
				Expect(hcsshim.ActivateLayer(driverInfo, containerId)).To(Succeed())
				Expect(hcsshim.PrepareLayer(driverInfo, containerId, sandboxLayers)).To(Succeed())
			})

			It("continues and creates the layer", func() {
				volumeGuid, err := client.CreateLayer(driverInfo, containerId, rootfsPath, sandboxLayers)
				Expect(err).ToNot(HaveOccurred())

				expectedVolumeGuid, err := hcsshim.GetLayerMountPath(driverInfo, containerId)
				Expect(err).ToNot(HaveOccurred())
				Expect(volumeGuid).ToNot(BeEmpty())
				Expect(volumeGuid).To(Equal(expectedVolumeGuid))
			})
		})
	})

	Describe("DestroyLayer", func() {
		Context("when the layer exists", func() {
			BeforeEach(func() {
				Expect(hcsshim.CreateSandboxLayer(driverInfo, containerId, rootfsPath, sandboxLayers)).To(Succeed())
				Expect(hcsshim.ActivateLayer(driverInfo, containerId)).To(Succeed())
				Expect(hcsshim.PrepareLayer(driverInfo, containerId, sandboxLayers)).To(Succeed())
			})

			It("destroys the layer", func() {
				Expect(client.DestroyLayer(driverInfo, containerId)).To(Succeed())
				Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeFalse())
			})
		})

		Context("when the layer exists but is not prepared", func() {
			BeforeEach(func() {
				Expect(hcsshim.CreateSandboxLayer(driverInfo, containerId, rootfsPath, sandboxLayers)).To(Succeed())
				Expect(hcsshim.ActivateLayer(driverInfo, containerId)).To(Succeed())
			})

			It("destroys the layer", func() {
				Expect(client.DestroyLayer(driverInfo, containerId)).To(Succeed())
				Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeFalse())
			})
		})

		Context("when the layer exists but is not activated", func() {
			BeforeEach(func() {
				Expect(hcsshim.CreateSandboxLayer(driverInfo, containerId, rootfsPath, sandboxLayers)).To(Succeed())
			})

			It("destroys the layer", func() {
				Expect(client.DestroyLayer(driverInfo, containerId)).To(Succeed())
				Expect(hcsshim.LayerExists(driverInfo, containerId)).To(BeFalse())
			})
		})

		Context("when the layer does not exist", func() {
			It("succeeds", func() {
				Expect(client.DestroyLayer(driverInfo, containerId)).To(Succeed())
			})
		})
	})

	Describe("Retryable", func() {
		Context("when the error is a timeout error", func() {
			It("returns true", func() {
				err := errors.New("Some operation failed: This operation returned because the timeout period expired")
				Expect(client.Retryable(err)).To(BeTrue())
			})
		})

		Context("when the error is something else", func() {
			It("returns false", func() {
				err := errors.New("some other error")
				Expect(client.Retryable(err)).To(BeFalse())
			})
		})
	})
})
