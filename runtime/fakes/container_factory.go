// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/runtime"
	"github.com/sirupsen/logrus"
)

type ContainerFactory struct {
	NewManagerStub        func(*logrus.Entry, *hcs.Client, string) runtime.ContainerManager
	newManagerMutex       sync.RWMutex
	newManagerArgsForCall []struct {
		arg1 *logrus.Entry
		arg2 *hcs.Client
		arg3 string
	}
	newManagerReturns struct {
		result1 runtime.ContainerManager
	}
	newManagerReturnsOnCall map[int]struct {
		result1 runtime.ContainerManager
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *ContainerFactory) NewManager(arg1 *logrus.Entry, arg2 *hcs.Client, arg3 string) runtime.ContainerManager {
	fake.newManagerMutex.Lock()
	ret, specificReturn := fake.newManagerReturnsOnCall[len(fake.newManagerArgsForCall)]
	fake.newManagerArgsForCall = append(fake.newManagerArgsForCall, struct {
		arg1 *logrus.Entry
		arg2 *hcs.Client
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.NewManagerStub
	fakeReturns := fake.newManagerReturns
	fake.recordInvocation("NewManager", []interface{}{arg1, arg2, arg3})
	fake.newManagerMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *ContainerFactory) NewManagerCallCount() int {
	fake.newManagerMutex.RLock()
	defer fake.newManagerMutex.RUnlock()
	return len(fake.newManagerArgsForCall)
}

func (fake *ContainerFactory) NewManagerCalls(stub func(*logrus.Entry, *hcs.Client, string) runtime.ContainerManager) {
	fake.newManagerMutex.Lock()
	defer fake.newManagerMutex.Unlock()
	fake.NewManagerStub = stub
}

func (fake *ContainerFactory) NewManagerArgsForCall(i int) (*logrus.Entry, *hcs.Client, string) {
	fake.newManagerMutex.RLock()
	defer fake.newManagerMutex.RUnlock()
	argsForCall := fake.newManagerArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *ContainerFactory) NewManagerReturns(result1 runtime.ContainerManager) {
	fake.newManagerMutex.Lock()
	defer fake.newManagerMutex.Unlock()
	fake.NewManagerStub = nil
	fake.newManagerReturns = struct {
		result1 runtime.ContainerManager
	}{result1}
}

func (fake *ContainerFactory) NewManagerReturnsOnCall(i int, result1 runtime.ContainerManager) {
	fake.newManagerMutex.Lock()
	defer fake.newManagerMutex.Unlock()
	fake.NewManagerStub = nil
	if fake.newManagerReturnsOnCall == nil {
		fake.newManagerReturnsOnCall = make(map[int]struct {
			result1 runtime.ContainerManager
		})
	}
	fake.newManagerReturnsOnCall[i] = struct {
		result1 runtime.ContainerManager
	}{result1}
}

func (fake *ContainerFactory) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.newManagerMutex.RLock()
	defer fake.newManagerMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *ContainerFactory) recordInvocation(key string, args []interface{}) {
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

var _ runtime.ContainerFactory = new(ContainerFactory)
