// Code generated by counterfeiter. DO NOT EDIT.
package networkfakes

import (
	"sync"

	"code.cloudfoundry.org/winc/netrules"
	"code.cloudfoundry.org/winc/network"
	"github.com/Microsoft/hcsshim"
)

type FakeNetRuleApplier struct {
	InStub        func(netrules.NetIn) (hcsshim.NatPolicy, error)
	inMutex       sync.RWMutex
	inArgsForCall []struct {
		arg1 netrules.NetIn
	}
	inReturns struct {
		result1 hcsshim.NatPolicy
		result2 error
	}
	inReturnsOnCall map[int]struct {
		result1 hcsshim.NatPolicy
		result2 error
	}
	OutStub        func(netrules.NetOut, hcsshim.HNSEndpoint) error
	outMutex       sync.RWMutex
	outArgsForCall []struct {
		arg1 netrules.NetOut
		arg2 hcsshim.HNSEndpoint
	}
	outReturns struct {
		result1 error
	}
	outReturnsOnCall map[int]struct {
		result1 error
	}
	NatMTUStub        func(int) error
	natMTUMutex       sync.RWMutex
	natMTUArgsForCall []struct {
		arg1 int
	}
	natMTUReturns struct {
		result1 error
	}
	natMTUReturnsOnCall map[int]struct {
		result1 error
	}
	ContainerMTUStub        func(int) error
	containerMTUMutex       sync.RWMutex
	containerMTUArgsForCall []struct {
		arg1 int
	}
	containerMTUReturns struct {
		result1 error
	}
	containerMTUReturnsOnCall map[int]struct {
		result1 error
	}
	CleanupStub        func() error
	cleanupMutex       sync.RWMutex
	cleanupArgsForCall []struct{}
	cleanupReturns     struct {
		result1 error
	}
	cleanupReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeNetRuleApplier) In(arg1 netrules.NetIn) (hcsshim.NatPolicy, error) {
	fake.inMutex.Lock()
	ret, specificReturn := fake.inReturnsOnCall[len(fake.inArgsForCall)]
	fake.inArgsForCall = append(fake.inArgsForCall, struct {
		arg1 netrules.NetIn
	}{arg1})
	fake.recordInvocation("In", []interface{}{arg1})
	fake.inMutex.Unlock()
	if fake.InStub != nil {
		return fake.InStub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.inReturns.result1, fake.inReturns.result2
}

func (fake *FakeNetRuleApplier) InCallCount() int {
	fake.inMutex.RLock()
	defer fake.inMutex.RUnlock()
	return len(fake.inArgsForCall)
}

func (fake *FakeNetRuleApplier) InArgsForCall(i int) netrules.NetIn {
	fake.inMutex.RLock()
	defer fake.inMutex.RUnlock()
	return fake.inArgsForCall[i].arg1
}

func (fake *FakeNetRuleApplier) InReturns(result1 hcsshim.NatPolicy, result2 error) {
	fake.InStub = nil
	fake.inReturns = struct {
		result1 hcsshim.NatPolicy
		result2 error
	}{result1, result2}
}

func (fake *FakeNetRuleApplier) InReturnsOnCall(i int, result1 hcsshim.NatPolicy, result2 error) {
	fake.InStub = nil
	if fake.inReturnsOnCall == nil {
		fake.inReturnsOnCall = make(map[int]struct {
			result1 hcsshim.NatPolicy
			result2 error
		})
	}
	fake.inReturnsOnCall[i] = struct {
		result1 hcsshim.NatPolicy
		result2 error
	}{result1, result2}
}

func (fake *FakeNetRuleApplier) Out(arg1 netrules.NetOut, arg2 hcsshim.HNSEndpoint) error {
	fake.outMutex.Lock()
	ret, specificReturn := fake.outReturnsOnCall[len(fake.outArgsForCall)]
	fake.outArgsForCall = append(fake.outArgsForCall, struct {
		arg1 netrules.NetOut
		arg2 hcsshim.HNSEndpoint
	}{arg1, arg2})
	fake.recordInvocation("Out", []interface{}{arg1, arg2})
	fake.outMutex.Unlock()
	if fake.OutStub != nil {
		return fake.OutStub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.outReturns.result1
}

func (fake *FakeNetRuleApplier) OutCallCount() int {
	fake.outMutex.RLock()
	defer fake.outMutex.RUnlock()
	return len(fake.outArgsForCall)
}

func (fake *FakeNetRuleApplier) OutArgsForCall(i int) (netrules.NetOut, hcsshim.HNSEndpoint) {
	fake.outMutex.RLock()
	defer fake.outMutex.RUnlock()
	return fake.outArgsForCall[i].arg1, fake.outArgsForCall[i].arg2
}

func (fake *FakeNetRuleApplier) OutReturns(result1 error) {
	fake.OutStub = nil
	fake.outReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeNetRuleApplier) OutReturnsOnCall(i int, result1 error) {
	fake.OutStub = nil
	if fake.outReturnsOnCall == nil {
		fake.outReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.outReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeNetRuleApplier) NatMTU(arg1 int) error {
	fake.natMTUMutex.Lock()
	ret, specificReturn := fake.natMTUReturnsOnCall[len(fake.natMTUArgsForCall)]
	fake.natMTUArgsForCall = append(fake.natMTUArgsForCall, struct {
		arg1 int
	}{arg1})
	fake.recordInvocation("NatMTU", []interface{}{arg1})
	fake.natMTUMutex.Unlock()
	if fake.NatMTUStub != nil {
		return fake.NatMTUStub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.natMTUReturns.result1
}

func (fake *FakeNetRuleApplier) NatMTUCallCount() int {
	fake.natMTUMutex.RLock()
	defer fake.natMTUMutex.RUnlock()
	return len(fake.natMTUArgsForCall)
}

func (fake *FakeNetRuleApplier) NatMTUArgsForCall(i int) int {
	fake.natMTUMutex.RLock()
	defer fake.natMTUMutex.RUnlock()
	return fake.natMTUArgsForCall[i].arg1
}

func (fake *FakeNetRuleApplier) NatMTUReturns(result1 error) {
	fake.NatMTUStub = nil
	fake.natMTUReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeNetRuleApplier) NatMTUReturnsOnCall(i int, result1 error) {
	fake.NatMTUStub = nil
	if fake.natMTUReturnsOnCall == nil {
		fake.natMTUReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.natMTUReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeNetRuleApplier) ContainerMTU(arg1 int) error {
	fake.containerMTUMutex.Lock()
	ret, specificReturn := fake.containerMTUReturnsOnCall[len(fake.containerMTUArgsForCall)]
	fake.containerMTUArgsForCall = append(fake.containerMTUArgsForCall, struct {
		arg1 int
	}{arg1})
	fake.recordInvocation("ContainerMTU", []interface{}{arg1})
	fake.containerMTUMutex.Unlock()
	if fake.ContainerMTUStub != nil {
		return fake.ContainerMTUStub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.containerMTUReturns.result1
}

func (fake *FakeNetRuleApplier) ContainerMTUCallCount() int {
	fake.containerMTUMutex.RLock()
	defer fake.containerMTUMutex.RUnlock()
	return len(fake.containerMTUArgsForCall)
}

func (fake *FakeNetRuleApplier) ContainerMTUArgsForCall(i int) int {
	fake.containerMTUMutex.RLock()
	defer fake.containerMTUMutex.RUnlock()
	return fake.containerMTUArgsForCall[i].arg1
}

func (fake *FakeNetRuleApplier) ContainerMTUReturns(result1 error) {
	fake.ContainerMTUStub = nil
	fake.containerMTUReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeNetRuleApplier) ContainerMTUReturnsOnCall(i int, result1 error) {
	fake.ContainerMTUStub = nil
	if fake.containerMTUReturnsOnCall == nil {
		fake.containerMTUReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.containerMTUReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeNetRuleApplier) Cleanup() error {
	fake.cleanupMutex.Lock()
	ret, specificReturn := fake.cleanupReturnsOnCall[len(fake.cleanupArgsForCall)]
	fake.cleanupArgsForCall = append(fake.cleanupArgsForCall, struct{}{})
	fake.recordInvocation("Cleanup", []interface{}{})
	fake.cleanupMutex.Unlock()
	if fake.CleanupStub != nil {
		return fake.CleanupStub()
	}
	if specificReturn {
		return ret.result1
	}
	return fake.cleanupReturns.result1
}

func (fake *FakeNetRuleApplier) CleanupCallCount() int {
	fake.cleanupMutex.RLock()
	defer fake.cleanupMutex.RUnlock()
	return len(fake.cleanupArgsForCall)
}

func (fake *FakeNetRuleApplier) CleanupReturns(result1 error) {
	fake.CleanupStub = nil
	fake.cleanupReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeNetRuleApplier) CleanupReturnsOnCall(i int, result1 error) {
	fake.CleanupStub = nil
	if fake.cleanupReturnsOnCall == nil {
		fake.cleanupReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.cleanupReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeNetRuleApplier) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.inMutex.RLock()
	defer fake.inMutex.RUnlock()
	fake.outMutex.RLock()
	defer fake.outMutex.RUnlock()
	fake.natMTUMutex.RLock()
	defer fake.natMTUMutex.RUnlock()
	fake.containerMTUMutex.RLock()
	defer fake.containerMTUMutex.RUnlock()
	fake.cleanupMutex.RLock()
	defer fake.cleanupMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeNetRuleApplier) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ network.NetRuleApplier = new(FakeNetRuleApplier)
