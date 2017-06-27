package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
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

	Context("stdin contains a port mapping request", func() {
		It("prints the correct port mapping for the container", func() {
			cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}]}`)
			output, err := cmd.CombinedOutput()
			Expect(err).To(Succeed())

			regexp := `{"properties":{"garden\.network\.container-ip":"\d+\.\d+\.\d+\.\d+","garden\.network\.host-ip":"255\.255\.255\.255","garden\.network\.mapped-ports":"\[{\\"host_port\\":\d+,\\"container_port\\":8080}\]"}}`
			Expect(string(output)).To(MatchRegexp(regexp))
		})
	})

	Context("stdin contains a port mapping request with two ports", func() {
		It("prints the correct port mapping for the container", func() {
			cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}, {"host_port": 0, "container_port": 2222}]}`)
			output, err := cmd.CombinedOutput()
			Expect(err).To(Succeed())

			regexp := `{"properties":{"garden\.network\.container-ip":"\d+\.\d+\.\d+\.\d+","garden\.network\.host-ip":"255\.255\.255\.255","garden\.network\.mapped-ports":"\[{\\"host_port\\":\d+,\\"container_port\\":8080},{\\"host_port\\":\d+,\\"container_port\\":2222}\]"}}`
			Expect(string(output)).To(MatchRegexp(regexp))
		})
	})

	Context("stdin does not contain a port mapping request", func() {
		It("prints an empty list of mapped ports", func() {
			cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} }`)
			output, err := cmd.CombinedOutput()
			Expect(err).To(Succeed())

			regexp := `{"properties":{"garden\.network\.container-ip":"\d+\.\d+\.\d+\.\d+","garden\.network\.host-ip":"255\.255\.255\.255","garden\.network\.mapped-ports":"\[\]"}}`
			Expect(string(output)).To(MatchRegexp(regexp))
		})
	})

	Context("stdin contains an invalid port mapping request", func() {
		It("errors", func() {
			cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 1234}, {"host_port": 0, "container_port": 2222}]}`)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).To(Succeed())

			Eventually(session.Err).Should(gbytes.Say("invalid port mapping"))
			Eventually(session).Should(gexec.Exit(1))
		})
	})
})