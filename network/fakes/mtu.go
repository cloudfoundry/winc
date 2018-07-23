// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"code.cloudfoundry.org/winc/network"
)

type Mtu struct {
	SetNatStub        func(int) error
	setNatMutex       sync.RWMutex
	setNatArgsForCall []struct {
		arg1 int
	}
	setNatReturns struct {
		result1 error
	}
	setNatReturnsOnCall map[int]struct {
		result1 error
	}
	SetContainerStub        func(int) error
	setContainerMutex       sync.RWMutex
	setContainerArgsForCall []struct {
		arg1 int
	}
	setContainerReturns struct {
		result1 error
	}
	setContainerReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *Mtu) SetNat(arg1 int) error {
	fake.setNatMutex.Lock()
	ret, specificReturn := fake.setNatReturnsOnCall[len(fake.setNatArgsForCall)]
	fake.setNatArgsForCall = append(fake.setNatArgsForCall, struct {
		arg1 int
	}{arg1})
	fake.recordInvocation("SetNat", []interface{}{arg1})
	fake.setNatMutex.Unlock()
	if fake.SetNatStub != nil {
		return fake.SetNatStub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.setNatReturns.result1
}

func (fake *Mtu) SetNatCallCount() int {
	fake.setNatMutex.RLock()
	defer fake.setNatMutex.RUnlock()
	return len(fake.setNatArgsForCall)
}

func (fake *Mtu) SetNatArgsForCall(i int) int {
	fake.setNatMutex.RLock()
	defer fake.setNatMutex.RUnlock()
	return fake.setNatArgsForCall[i].arg1
}

func (fake *Mtu) SetNatReturns(result1 error) {
	fake.SetNatStub = nil
	fake.setNatReturns = struct {
		result1 error
	}{result1}
}

func (fake *Mtu) SetNatReturnsOnCall(i int, result1 error) {
	fake.SetNatStub = nil
	if fake.setNatReturnsOnCall == nil {
		fake.setNatReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.setNatReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *Mtu) SetContainer(arg1 int) error {
	fake.setContainerMutex.Lock()
	ret, specificReturn := fake.setContainerReturnsOnCall[len(fake.setContainerArgsForCall)]
	fake.setContainerArgsForCall = append(fake.setContainerArgsForCall, struct {
		arg1 int
	}{arg1})
	fake.recordInvocation("SetContainer", []interface{}{arg1})
	fake.setContainerMutex.Unlock()
	if fake.SetContainerStub != nil {
		return fake.SetContainerStub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.setContainerReturns.result1
}

func (fake *Mtu) SetContainerCallCount() int {
	fake.setContainerMutex.RLock()
	defer fake.setContainerMutex.RUnlock()
	return len(fake.setContainerArgsForCall)
}

func (fake *Mtu) SetContainerArgsForCall(i int) int {
	fake.setContainerMutex.RLock()
	defer fake.setContainerMutex.RUnlock()
	return fake.setContainerArgsForCall[i].arg1
}

func (fake *Mtu) SetContainerReturns(result1 error) {
	fake.SetContainerStub = nil
	fake.setContainerReturns = struct {
		result1 error
	}{result1}
}

func (fake *Mtu) SetContainerReturnsOnCall(i int, result1 error) {
	fake.SetContainerStub = nil
	if fake.setContainerReturnsOnCall == nil {
		fake.setContainerReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.setContainerReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *Mtu) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.setNatMutex.RLock()
	defer fake.setNatMutex.RUnlock()
	fake.setContainerMutex.RLock()
	defer fake.setContainerMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *Mtu) recordInvocation(key string, args []interface{}) {
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

var _ network.Mtu = new(Mtu)