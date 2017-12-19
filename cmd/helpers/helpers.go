package helpers_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	mathrand "math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/filelock"
	"code.cloudfoundry.org/winc/network"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type Helpers struct {
	wincBin         string
	wincImageBin    string
	wincNetworkBin  string
	gatewayFileName string
}

func NewHelpers(wincBin, wincImageBin, wincNetworkBin string) *Helpers {
	return &Helpers{
		wincBin:         wincBin,
		wincImageBin:    wincImageBin,
		wincNetworkBin:  wincNetworkBin,
		gatewayFileName: "c:\\var\\vcap\\data\\winc-network\\gateways.json",
	}
}

func (h *Helpers) GetContainerState(containerId string) specs.State {
	stdOut, _, err := h.Execute(exec.Command(h.wincBin, "state", containerId))
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	var state specs.State
	ExpectWithOffset(1, json.Unmarshal(stdOut.Bytes(), &state)).To(Succeed())
	return state
}

func (h *Helpers) GenerateBundle(bundleSpec specs.Spec, bundlePath string) {
	config, err := json.Marshal(&bundleSpec)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	configFile := filepath.Join(bundlePath, "config.json")
	ExpectWithOffset(1, ioutil.WriteFile(configFile, config, 0666)).To(Succeed())
}

func (h *Helpers) CreateContainer(bundleSpec specs.Spec, bundlePath, containerId string) {
	h.GenerateBundle(bundleSpec, bundlePath)
	_, _, err := h.Execute(exec.Command(h.wincBin, "create", "-b", bundlePath, containerId))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func (h *Helpers) DeleteContainer(id string) {
	if h.ContainerExists(id) {
		output, err := exec.Command(h.wincBin, "delete", id).CombinedOutput()
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), string(output))
	}
}

func (h *Helpers) CreateSandbox(storePath, rootfsPath, containerId string) specs.Spec {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	cmd := exec.Command(h.wincImageBin, "--store", storePath, "create", rootfsPath, containerId)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	ExpectWithOffset(1, cmd.Run()).To(Succeed(), fmt.Sprintf("winc-image stdout: %s\n\n winc-image stderr: %s\n\n", stdOut.String(), stdErr.String()))
	var spec specs.Spec
	ExpectWithOffset(1, json.Unmarshal(stdOut.Bytes(), &spec)).To(Succeed())
	return spec
}

func (h *Helpers) DeleteSandbox(imageStore, id string) {
	output, err := exec.Command(h.wincImageBin, "--store", imageStore, "delete", id).CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), string(output))
}

func (h *Helpers) CreateNetwork(networkConfig network.Config, networkConfigFile string, extraArgs ...string) {
	file, err := os.Create(networkConfigFile)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	data, err := json.Marshal(networkConfig)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	_, err = file.Write(data)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	file.Close()

	args := append([]string{"--action", "create", "--configFile", networkConfigFile})
	args = append(args, extraArgs...)
	_, _, err = h.Execute(exec.Command(h.wincNetworkBin, args...))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func (h *Helpers) DeleteNetwork(networkConfig network.Config, networkConfigFile string) {
	gatewayFile := filelock.NewLocker(h.gatewayFileName)
	f, err := gatewayFile.Open()
	defer f.Close()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	oldGatewaysInUse := h.loadGatewaysInUse(f)
	var newGatewaysInUse []string

	for _, gateway := range oldGatewaysInUse {
		if gateway != networkConfig.GatewayAddress {
			newGatewaysInUse = append(newGatewaysInUse, gateway)
		}
	}

	h.writeGatewaysInUse(f, newGatewaysInUse)
	args := []string{"--action", "delete", "--configFile", networkConfigFile}
	_, _, err = h.Execute(exec.Command(h.wincNetworkBin, args...))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func (h *Helpers) NetworkUp(id, input, networkConfigFile string) network.UpOutputs {
	args := []string{"--action", "up", "--configFile", networkConfigFile, "--handle", id}
	cmd := exec.Command(h.wincNetworkBin, args...)
	cmd.Stdin = strings.NewReader(input)
	stdOut, _, err := h.Execute(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	var upOutput network.UpOutputs
	ExpectWithOffset(1, json.Unmarshal(stdOut.Bytes(), &upOutput)).To(Succeed())
	return upOutput
}

func (h *Helpers) NetworkDown(id, networkConfigFile string) {
	_, _, err := h.Execute(exec.Command(h.wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", id))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func (h *Helpers) GenerateNetworkConfig() network.Config {
	var subnet, gateway string

	gatewayFile := filelock.NewLocker(h.gatewayFileName)
	f, err := gatewayFile.Open()
	defer f.Close()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	gatewaysInUse := h.loadGatewaysInUse(f)

	for {
		subnet, gateway = h.randomValidSubnetAddress()
		if !h.natNetworkInUse(gateway, gatewaysInUse) && !h.collideWithHost(gateway) {
			gatewaysInUse = append(gatewaysInUse, gateway)
			break
		}
	}

	h.writeGatewaysInUse(f, gatewaysInUse)

	return network.Config{
		SubnetRange:    subnet,
		GatewayAddress: gateway,
		NetworkName:    gateway,
	}
}

func (h *Helpers) ContainerExists(containerId string) bool {
	query := hcsshim.ComputeSystemQuery{
		Owners: []string{"winc"},
		IDs:    []string{containerId},
	}
	containers, err := hcsshim.GetContainers(query)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	return len(containers) > 0
}

func (h *Helpers) ExecInContainer(id string, args []string, detach bool) (*bytes.Buffer, *bytes.Buffer, error) {
	var defaultArgs []string

	if detach {
		defaultArgs = []string{"exec", "-u", "vcap", "-d", id}
	} else {
		defaultArgs = []string{"exec", "-u", "vcap", id}
	}

	return h.Execute(exec.Command(h.wincBin, append(defaultArgs, args...)...))
}

func (h *Helpers) GenerateRuntimeSpec(baseSpec specs.Spec) specs.Spec {
	return specs.Spec{
		Version: specs.Version,
		Process: &specs.Process{
			Args: []string{"powershell"},
			Cwd:  "C:\\",
		},
		Root: &specs.Root{
			Path: baseSpec.Root.Path,
		},
		Windows: &specs.Windows{
			LayerFolders: baseSpec.Windows.LayerFolders,
		},
	}
}

func (h *Helpers) RandomContainerId() string {
	max := big.NewInt(math.MaxInt64)
	r, err := rand.Int(rand.Reader, max)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return fmt.Sprintf("%d", r.Int64())
}

func (h *Helpers) CopyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}

func (h *Helpers) Execute(c *exec.Cmd) (*bytes.Buffer, *bytes.Buffer, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	c.Stdout = io.MultiWriter(stdOut, GinkgoWriter)
	c.Stderr = io.MultiWriter(stdErr, GinkgoWriter)
	err := c.Run()

	return stdOut, stdErr, err
}

func (h *Helpers) ExitCode(err error) (int, error) {
	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus(), nil
		} else {
			return -1, errors.New("Error did not have a syscall.WaitStatus")
		}
	} else {
		return -1, errors.New("Error was not an exec.ExitError")
	}
}

func (h *Helpers) loadGatewaysInUse(f filelock.LockedFile) []string {
	data := make([]byte, 10240)
	n, err := f.Read(data)
	if err != nil {
		ExpectWithOffset(2, err).To(Equal(io.EOF))
		data = []byte("[]")
		n = 2
	}

	gateways := []string{}
	ExpectWithOffset(2, json.Unmarshal(data[:n], &gateways)).To(Succeed())

	return gateways
}

func (h *Helpers) writeGatewaysInUse(f filelock.LockedFile, gateways []string) {
	data, err := json.Marshal(gateways)
	ExpectWithOffset(2, err).NotTo(HaveOccurred())

	_, err = f.Seek(0, io.SeekStart)
	ExpectWithOffset(2, err).NotTo(HaveOccurred())
	ExpectWithOffset(2, f.Truncate(0)).To(Succeed())

	_, err = f.Write(data)
	ExpectWithOffset(2, err).NotTo(HaveOccurred())
}

func (h *Helpers) natNetworkInUse(name string, inuse []string) bool {
	for _, n := range inuse {
		if name == n {
			return true
		}
	}

	_, err := hcsshim.GetHNSNetworkByName(name)
	if err != nil {
		ExpectWithOffset(2, err).To(MatchError(ContainSubstring("Network " + name + " not found")))
		return false
	}

	return true
}

func (h *Helpers) collideWithHost(gateway string) bool {
	hostip, err := localip.LocalIP()
	ExpectWithOffset(2, err).NotTo(HaveOccurred())

	hostbytes := strings.Split(hostip, ".")
	gatewaybytes := strings.Split(gateway, ".")

	// only need to compare first 3 bytes since mask is /24
	return hostbytes[0] == gatewaybytes[0] &&
		hostbytes[1] == gatewaybytes[1] &&
		hostbytes[2] == gatewaybytes[2]
}

func (h *Helpers) randomValidSubnetAddress() (string, string) {
	randomOctet := mathrand.Intn(256)
	gatewayAddress := fmt.Sprintf("172.16.%d.1", randomOctet)
	subnet := fmt.Sprintf("172.16.%d.0/24", randomOctet)
	return subnet, gatewayAddress
}
