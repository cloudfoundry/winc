package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("up", func() {
	var (
		config      []byte
		containerId string
		bundleSpec  specs.Spec
		err         error
	)

	BeforeEach(func() {
		containerId = filepath.Base(bundlePath)
		bundleSpec = runtimeSpecGenerator(rootfsPath)
		config, err = json.Marshal(&bundleSpec)
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0666)).To(Succeed())

		err := exec.Command(wincBin, "create", "-b", bundlePath, containerId).Run()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := exec.Command(wincBin, "delete", containerId).Run()
		Expect(err).ToNot(HaveOccurred())
	})

	It("prints the correct port mapping for the container", func() {
		cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
		cmd.Stdin = strings.NewReader(`{"netin": [{"host_port": 0, "container_port": 8080}] }`)
		output, err := cmd.CombinedOutput()
		Expect(err).To(Succeed())

		regexp := `{"properties":{"garden\.network\.container-ip":"\d+\.\d+\.\d+\.\d+","garden\.network\.host-ip":"255\.255\.255\.255","garden\.network\.mapped-ports":"{\\"host_port\\":\d+,\\"container_port\\":8080}"}}`
		Expect(string(output)).To(MatchRegexp(regexp))
	})
})
