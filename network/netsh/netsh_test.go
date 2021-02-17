package netsh_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"time"

	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"code.cloudfoundry.org/winc/network/netsh"
	"code.cloudfoundry.org/winc/network/netsh/fakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Netsh", func() {
	const containerId = "container123"

	var (
		runner    *netsh.Runner
		hcsClient *fakes.HCSClient
	)

	BeforeEach(func() {
		hcsClient = &fakes.HCSClient{}
		runner = netsh.NewRunner(hcsClient, containerId, 2)
		logrus.SetOutput(ioutil.Discard)
	})

	Describe("RunContainer", func() {
		var (
			fakeContainer *hcsfakes.Container
			fakeProcess   *hcsfakes.Process
		)

		BeforeEach(func() {
			fakeContainer = &hcsfakes.Container{}
			hcsClient.OpenContainerReturns(fakeContainer, nil)
			fakeProcess = &hcsfakes.Process{}
			fakeContainer.CreateProcessReturns(fakeProcess, nil)
		})

		It("runs a netsh command in the specified container", func() {
			Expect(runner.RunContainer([]string{"some", "command"})).To(Succeed())

			Expect(hcsClient.OpenContainerCallCount()).To(Equal(1))
			Expect(hcsClient.OpenContainerArgsForCall(0)).To(Equal(containerId))

			Expect(fakeContainer.CreateProcessCallCount()).To(Equal(1))
			expectedProcessConfig := hcsshim.ProcessConfig{
				CommandLine: "netsh some command",
			}
			Expect(*fakeContainer.CreateProcessArgsForCall(0)).To(Equal(expectedProcessConfig))

			Expect(fakeProcess.WaitTimeoutCallCount()).To(Equal(1))
			Expect(fakeProcess.WaitTimeoutArgsForCall(0)).To(Equal(time.Second * 2))

			Expect(fakeProcess.ExitCodeCallCount()).To(Equal(1))

			Expect(fakeContainer.CloseCallCount()).To(Equal(1))
		})

		It("writes the netsh command line to the log", func() {
			buffer := new(bytes.Buffer)
			logrus.SetOutput(buffer)

			Expect(runner.RunContainer([]string{"some", "command"})).To(Succeed())
			Expect(buffer.String()).To(ContainSubstring("running 'netsh some command' in container123"))
		})

		Context("netsh fails with a nonzero exit code", func() {
			BeforeEach(func() {
				fakeProcess.ExitCodeReturns(1, nil)
			})

			It("returns an error", func() {
				err := runner.RunContainer([]string{"some", "command"})
				Expect(err).To(MatchError(errors.New("running 'netsh some command' in container123 failed: exit code 1")))
				Expect(fakeContainer.CloseCallCount()).To(Equal(1))
			})

			It("writes the netsh command line and exit code to the log", func() {
				buffer := new(bytes.Buffer)
				logrus.SetOutput(buffer)

				runner.RunContainer([]string{"some", "command"})
				Expect(buffer.String()).To(ContainSubstring("running 'netsh some command' in container123 failed: exit code 1"))
			})
		})
	})
})
