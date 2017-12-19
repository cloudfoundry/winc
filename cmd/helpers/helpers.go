package helpers_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"syscall"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type Helpers struct {
	WincBin        string
	WincImageBin   string
	WincNetworkBin string
}

func (h *Helpers) GetContainerState(containerId string) specs.State {
	stdOut, _, err := h.Execute(exec.Command(h.WincBin, "state", containerId))
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	var state specs.State
	ExpectWithOffset(1, json.Unmarshal(stdOut.Bytes(), &state)).To(Succeed())
	return state
}

func (h *Helpers) DeleteContainer(id string) {
	if h.ContainerExists(id) {
		output, err := exec.Command(h.WincBin, "delete", id).CombinedOutput()
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), string(output))
	}
}

func (h *Helpers) CreateSandbox(storePath, rootfsPath, containerId string) specs.Spec {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	cmd := exec.Command(h.WincImageBin, "--store", storePath, "create", rootfsPath, containerId)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	ExpectWithOffset(1, cmd.Run()).To(Succeed(), fmt.Sprintf("winc-image stdout: %s\n\n winc-image stderr: %s\n\n", stdOut.String(), stdErr.String()))
	var spec specs.Spec
	ExpectWithOffset(1, json.Unmarshal(stdOut.Bytes(), &spec)).To(Succeed())
	return spec
}

func (h *Helpers) DeleteSandbox(imageStore, id string) {
	output, err := exec.Command(h.WincImageBin, "--store", imageStore, "delete", id).CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), string(output))
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

	return h.Execute(exec.Command(h.WincBin, append(defaultArgs, args...)...))
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
