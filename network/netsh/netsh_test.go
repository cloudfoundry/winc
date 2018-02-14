package netsh_test

import (
	"errors"
	"time"

	hcsfakes "code.cloudfoundry.org/winc/hcs/fakes"
	"code.cloudfoundry.org/winc/network/netsh"
	"code.cloudfoundry.org/winc/network/netsh/fakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Netsh", func() {
	const containerId = "container123"

	var (
		runner    *netsh.Runner
		hcsClient *fakes.HCSClient
	)

	BeforeEach(func() {
		hcsClient = &fakes.HCSClient{}
		runner = netsh.NewRunner(hcsClient, containerId)
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
})
