package hcs_test

import (
	"errors"
	"syscall"

	"code.cloudfoundry.org/winc/hcs"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HCS Errors", func() {
	Describe("CleanError", func() {
		Context("when given an hcsshim low memory error", func() {
			var inputError error
			BeforeEach(func() {
				inputError = &hcsshim.ContainerError{
					Err: syscall.Errno(0x5af),
				}
			})

			It("returns a LowMemoryError", func() {
				outputError := hcs.CleanError(inputError)
				Expect(outputError).To(BeAssignableToTypeOf(&hcs.LowMemoryError{}))
			})
		})
		Context("when given an hcsshim other error", func() {
			var inputError error
			BeforeEach(func() {
				inputError = &hcsshim.ContainerError{
					Err: syscall.Errno(0x5ae),
				}
			})

			It("returns the internal error", func() {
				outputError := hcs.CleanError(inputError)
				Expect(outputError).To(Equal(syscall.Errno(0x5ae)))
			})
		})
		Context("when given a regular error", func() {
			var inputError error
			BeforeEach(func() {
				inputError = errors.New("some error")
			})

			It("returns the original error", func() {
				outputError := hcs.CleanError(inputError)
				Expect(outputError).To(Equal(inputError))
			})
		})
	})
})
