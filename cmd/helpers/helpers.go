package helpers_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func GetContainerState(binaryPath, containerId string) specs.State {
	stdOut, _, err := Execute(exec.Command(binaryPath, "state", containerId))
	Expect(err).ToNot(HaveOccurred())

	var state specs.State
	Expect(json.Unmarshal(stdOut.Bytes(), &state)).To(Succeed())
	return state
}

func DeleteContainer(binaryPath, id string) {
	if ContainerExists(id) {
		output, err := exec.Command(binaryPath, "delete", id).CombinedOutput()
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), string(output))
	}
}

func CreateSandbox(binaryPath, storePath, rootfsPath, containerId string) specs.Spec {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	cmd := exec.Command(binaryPath, "--store", storePath, "create", rootfsPath, containerId)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	Expect(cmd.Run()).To(Succeed(), fmt.Sprintf("winc-image stdout: %s\n\n winc-image stderr: %s\n\n", stdOut.String(), stdErr.String()))
	var spec specs.Spec
	Expect(json.Unmarshal(stdOut.Bytes(), &spec)).To(Succeed())
	return spec
}

func DeleteSandbox(binaryPath, imageStore, id string) {
	output, err := exec.Command(binaryPath, "--store", imageStore, "delete", id).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func ContainerExists(containerId string) bool {
	query := hcsshim.ComputeSystemQuery{
		Owners: []string{"winc"},
		IDs:    []string{containerId},
	}
	containers, err := hcsshim.GetContainers(query)
	Expect(err).ToNot(HaveOccurred())
	return len(containers) > 0
}

func ExecInContainer(binaryPath, id string, args []string, detach bool) (*bytes.Buffer, *bytes.Buffer, error) {
	var defaultArgs []string

	if detach {
		defaultArgs = []string{"exec", "-u", "vcap", "-d", id}
	} else {
		defaultArgs = []string{"exec", "-u", "vcap", id}
	}

	return Execute(exec.Command(binaryPath, append(defaultArgs, args...)...))
}

func GenerateRuntimeSpec(baseSpec specs.Spec) specs.Spec {
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

func RandomContainerId() string {
	max := big.NewInt(math.MaxInt64)
	r, err := rand.Int(rand.Reader, max)
	Expect(err).NotTo(HaveOccurred())

	return fmt.Sprintf("%d", r.Int64())
}

func CopyFile(dst, src string) error {
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

func Execute(c *exec.Cmd) (*bytes.Buffer, *bytes.Buffer, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	c.Stdout = io.MultiWriter(stdOut, GinkgoWriter)
	c.Stderr = io.MultiWriter(stdErr, GinkgoWriter)
	err := c.Run()

	return stdOut, stdErr, err
}
