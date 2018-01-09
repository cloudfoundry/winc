package netsh_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/winc/hcs/hcsfakes"
	"code.cloudfoundry.org/winc/network/netsh"
	"code.cloudfoundry.org/winc/network/netsh/netshfakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Netsh", func() {
	const containerId = "container123"

	var (
		runner    *netsh.Runner
		hcsClient *netshfakes.FakeHCSClient
	)

	BeforeEach(func() {
		hcsClient = &netshfakes.FakeHCSClient{}
		runner = netsh.NewRunner(hcsClient, containerId)
	})

	Describe("RunContainer", func() {
		var (
			fakeContainer *hcsfakes.FakeContainer
			fakeProcess   *hcsfakes.FakeProcess
		)

		BeforeEach(func() {
			fakeContainer = &hcsfakes.FakeContainer{}
			hcsClient.OpenContainerReturns(fakeContainer, nil)
			fakeProcess = &hcsfakes.FakeProcess{}
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

		Context("netsh fails with a nonzero exit code", func() {
			BeforeEach(func() {
				fakeProcess.ExitCodeReturns(1, nil)
			})

			It("returns an error", func() {
				err := runner.RunContainer([]string{"some", "command"})
				Expect(err).To(MatchError(errors.New("failed to exec netsh some command in container container123: exit code 1")))
				Expect(fakeContainer.CloseCallCount()).To(Equal(1))
			})
		})
	})

	Describe("RunHost", func() {
		It("runs a netsh command on the host", func() {
			output, err := runner.RunHost([]string{"interface", "show", "interface"})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(output)).To(ContainSubstring("Interface Name"))
		})

		Context("when the command fails", func() {
			It("returns an error including the output", func() {
				errorMsg := "The following command was not found: some-bad-command."
				output, err := runner.RunHost([]string{"some-bad-command"})
				Expect(err).To(MatchError(ContainSubstring(errorMsg)))
				Expect(string(output)).To(ContainSubstring(errorMsg))
			})
		})
	})
})
