package main_test

import (
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"testing"
)

var wincBin string

func TestWinc(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeEach(func() {
		var err error
		wincBin, err = gexec.Build("code.cloudfoundry.org/winc/cmd/winc")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		gexec.CleanupBuildArtifacts()
	})

	RunSpecs(t, "Winc Suite")
}

func specGenerator(rootfsPath string) specs.Spec {
	return specs.Spec{
		Version: specs.Version,
		Platform: specs.Platform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		Process: &specs.Process{
			Args: []string{"powershell"},
			Cwd:  "/",
		},
		Root: specs.Root{
			Path: rootfsPath,
		},
	}
}
