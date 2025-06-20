// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"code.cloudfoundry.org/winc/network"
	"github.com/Microsoft/hcsshim"
)

type EndpointManager struct {
	ApplyPoliciesStub        func(hcsshim.HNSEndpoint, []*hcsshim.NatPolicy, []*hcsshim.ACLPolicy) (hcsshim.HNSEndpoint, error)
	applyPoliciesMutex       sync.RWMutex
	applyPoliciesArgsForCall []struct {
		arg1 hcsshim.HNSEndpoint
		arg2 []*hcsshim.NatPolicy
		arg3 []*hcsshim.ACLPolicy
	}
	applyPoliciesReturns struct {
		result1 hcsshim.HNSEndpoint
		result2 error
	}
	applyPoliciesReturnsOnCall map[int]struct {
		result1 hcsshim.HNSEndpoint
		result2 error
	}
	CreateStub        func() (hcsshim.HNSEndpoint, error)
	createMutex       sync.RWMutex
	createArgsForCall []struct {
	}
	createReturns struct {
		result1 hcsshim.HNSEndpoint
		result2 error
	}
	createReturnsOnCall map[int]struct {
		result1 hcsshim.HNSEndpoint
		result2 error
	}
	DeleteStub        func() error
	deleteMutex       sync.RWMutex
	deleteArgsForCall []struct {
	}
	deleteReturns struct {
		result1 error
	}
	deleteReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *EndpointManager) ApplyPolicies(arg1 hcsshim.HNSEndpoint, arg2 []*hcsshim.NatPolicy, arg3 []*hcsshim.ACLPolicy) (hcsshim.HNSEndpoint, error) {
	var arg2Copy []*hcsshim.NatPolicy
	if arg2 != nil {
		arg2Copy = make([]*hcsshim.NatPolicy, len(arg2))
		copy(arg2Copy, arg2)
	}
	var arg3Copy []*hcsshim.ACLPolicy
	if arg3 != nil {
		arg3Copy = make([]*hcsshim.ACLPolicy, len(arg3))
		copy(arg3Copy, arg3)
	}
	fake.applyPoliciesMutex.Lock()
	ret, specificReturn := fake.applyPoliciesReturnsOnCall[len(fake.applyPoliciesArgsForCall)]
	fake.applyPoliciesArgsForCall = append(fake.applyPoliciesArgsForCall, struct {
		arg1 hcsshim.HNSEndpoint
		arg2 []*hcsshim.NatPolicy
		arg3 []*hcsshim.ACLPolicy
	}{arg1, arg2Copy, arg3Copy})
	stub := fake.ApplyPoliciesStub
	fakeReturns := fake.applyPoliciesReturns
	fake.recordInvocation("ApplyPolicies", []interface{}{arg1, arg2Copy, arg3Copy})
	fake.applyPoliciesMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *EndpointManager) ApplyPoliciesCallCount() int {
	fake.applyPoliciesMutex.RLock()
	defer fake.applyPoliciesMutex.RUnlock()
	return len(fake.applyPoliciesArgsForCall)
}

func (fake *EndpointManager) ApplyPoliciesCalls(stub func(hcsshim.HNSEndpoint, []*hcsshim.NatPolicy, []*hcsshim.ACLPolicy) (hcsshim.HNSEndpoint, error)) {
	fake.applyPoliciesMutex.Lock()
	defer fake.applyPoliciesMutex.Unlock()
	fake.ApplyPoliciesStub = stub
}

func (fake *EndpointManager) ApplyPoliciesArgsForCall(i int) (hcsshim.HNSEndpoint, []*hcsshim.NatPolicy, []*hcsshim.ACLPolicy) {
	fake.applyPoliciesMutex.RLock()
	defer fake.applyPoliciesMutex.RUnlock()
	argsForCall := fake.applyPoliciesArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *EndpointManager) ApplyPoliciesReturns(result1 hcsshim.HNSEndpoint, result2 error) {
	fake.applyPoliciesMutex.Lock()
	defer fake.applyPoliciesMutex.Unlock()
	fake.ApplyPoliciesStub = nil
	fake.applyPoliciesReturns = struct {
		result1 hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *EndpointManager) ApplyPoliciesReturnsOnCall(i int, result1 hcsshim.HNSEndpoint, result2 error) {
	fake.applyPoliciesMutex.Lock()
	defer fake.applyPoliciesMutex.Unlock()
	fake.ApplyPoliciesStub = nil
	if fake.applyPoliciesReturnsOnCall == nil {
		fake.applyPoliciesReturnsOnCall = make(map[int]struct {
			result1 hcsshim.HNSEndpoint
			result2 error
		})
	}
	fake.applyPoliciesReturnsOnCall[i] = struct {
		result1 hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *EndpointManager) Create() (hcsshim.HNSEndpoint, error) {
	fake.createMutex.Lock()
	ret, specificReturn := fake.createReturnsOnCall[len(fake.createArgsForCall)]
	fake.createArgsForCall = append(fake.createArgsForCall, struct {
	}{})
	stub := fake.CreateStub
	fakeReturns := fake.createReturns
	fake.recordInvocation("Create", []interface{}{})
	fake.createMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *EndpointManager) CreateCallCount() int {
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	return len(fake.createArgsForCall)
}

func (fake *EndpointManager) CreateCalls(stub func() (hcsshim.HNSEndpoint, error)) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = stub
}

func (fake *EndpointManager) CreateReturns(result1 hcsshim.HNSEndpoint, result2 error) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = nil
	fake.createReturns = struct {
		result1 hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *EndpointManager) CreateReturnsOnCall(i int, result1 hcsshim.HNSEndpoint, result2 error) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = nil
	if fake.createReturnsOnCall == nil {
		fake.createReturnsOnCall = make(map[int]struct {
			result1 hcsshim.HNSEndpoint
			result2 error
		})
	}
	fake.createReturnsOnCall[i] = struct {
		result1 hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *EndpointManager) Delete() error {
	fake.deleteMutex.Lock()
	ret, specificReturn := fake.deleteReturnsOnCall[len(fake.deleteArgsForCall)]
	fake.deleteArgsForCall = append(fake.deleteArgsForCall, struct {
	}{})
	stub := fake.DeleteStub
	fakeReturns := fake.deleteReturns
	fake.recordInvocation("Delete", []interface{}{})
	fake.deleteMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *EndpointManager) DeleteCallCount() int {
	fake.deleteMutex.RLock()
	defer fake.deleteMutex.RUnlock()
	return len(fake.deleteArgsForCall)
}

func (fake *EndpointManager) DeleteCalls(stub func() error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = stub
}

func (fake *EndpointManager) DeleteReturns(result1 error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = nil
	fake.deleteReturns = struct {
		result1 error
	}{result1}
}

func (fake *EndpointManager) DeleteReturnsOnCall(i int, result1 error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = nil
	if fake.deleteReturnsOnCall == nil {
		fake.deleteReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *EndpointManager) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.applyPoliciesMutex.RLock()
	defer fake.applyPoliciesMutex.RUnlock()
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	fake.deleteMutex.RLock()
	defer fake.deleteMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *EndpointManager) recordInvocation(key string, args []interface{}) {
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

var _ network.EndpointManager = new(EndpointManager)
