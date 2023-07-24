// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"code.cloudfoundry.org/winc/network/netrules"
)

type NetShRunner struct {
	RunContainerStub        func([]string) error
	runContainerMutex       sync.RWMutex
	runContainerArgsForCall []struct {
		arg1 []string
	}
	runContainerReturns struct {
		result1 error
	}
	runContainerReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *NetShRunner) RunContainer(arg1 []string) error {
	var arg1Copy []string
	if arg1 != nil {
		arg1Copy = make([]string, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.runContainerMutex.Lock()
	ret, specificReturn := fake.runContainerReturnsOnCall[len(fake.runContainerArgsForCall)]
	fake.runContainerArgsForCall = append(fake.runContainerArgsForCall, struct {
		arg1 []string
	}{arg1Copy})
	stub := fake.RunContainerStub
	fakeReturns := fake.runContainerReturns
	fake.recordInvocation("RunContainer", []interface{}{arg1Copy})
	fake.runContainerMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *NetShRunner) RunContainerCallCount() int {
	fake.runContainerMutex.RLock()
	defer fake.runContainerMutex.RUnlock()
	return len(fake.runContainerArgsForCall)
}

func (fake *NetShRunner) RunContainerCalls(stub func([]string) error) {
	fake.runContainerMutex.Lock()
	defer fake.runContainerMutex.Unlock()
	fake.RunContainerStub = stub
}

func (fake *NetShRunner) RunContainerArgsForCall(i int) []string {
	fake.runContainerMutex.RLock()
	defer fake.runContainerMutex.RUnlock()
	argsForCall := fake.runContainerArgsForCall[i]
	return argsForCall.arg1
}

func (fake *NetShRunner) RunContainerReturns(result1 error) {
	fake.runContainerMutex.Lock()
	defer fake.runContainerMutex.Unlock()
	fake.RunContainerStub = nil
	fake.runContainerReturns = struct {
		result1 error
	}{result1}
}

func (fake *NetShRunner) RunContainerReturnsOnCall(i int, result1 error) {
	fake.runContainerMutex.Lock()
	defer fake.runContainerMutex.Unlock()
	fake.RunContainerStub = nil
	if fake.runContainerReturnsOnCall == nil {
		fake.runContainerReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.runContainerReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *NetShRunner) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.runContainerMutex.RLock()
	defer fake.runContainerMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *NetShRunner) recordInvocation(key string, args []interface{}) {
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

var _ netrules.NetShRunner = new(NetShRunner)
