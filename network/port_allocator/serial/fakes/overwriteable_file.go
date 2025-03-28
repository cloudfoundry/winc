// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"code.cloudfoundry.org/winc/network/port_allocator/serial"
)

type OverwriteableFile struct {
	ReadStub        func([]byte) (int, error)
	readMutex       sync.RWMutex
	readArgsForCall []struct {
		arg1 []byte
	}
	readReturns struct {
		result1 int
		result2 error
	}
	readReturnsOnCall map[int]struct {
		result1 int
		result2 error
	}
	SeekStub        func(int64, int) (int64, error)
	seekMutex       sync.RWMutex
	seekArgsForCall []struct {
		arg1 int64
		arg2 int
	}
	seekReturns struct {
		result1 int64
		result2 error
	}
	seekReturnsOnCall map[int]struct {
		result1 int64
		result2 error
	}
	TruncateStub        func(int64) error
	truncateMutex       sync.RWMutex
	truncateArgsForCall []struct {
		arg1 int64
	}
	truncateReturns struct {
		result1 error
	}
	truncateReturnsOnCall map[int]struct {
		result1 error
	}
	WriteStub        func([]byte) (int, error)
	writeMutex       sync.RWMutex
	writeArgsForCall []struct {
		arg1 []byte
	}
	writeReturns struct {
		result1 int
		result2 error
	}
	writeReturnsOnCall map[int]struct {
		result1 int
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *OverwriteableFile) Read(arg1 []byte) (int, error) {
	var arg1Copy []byte
	if arg1 != nil {
		arg1Copy = make([]byte, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.readMutex.Lock()
	ret, specificReturn := fake.readReturnsOnCall[len(fake.readArgsForCall)]
	fake.readArgsForCall = append(fake.readArgsForCall, struct {
		arg1 []byte
	}{arg1Copy})
	stub := fake.ReadStub
	fakeReturns := fake.readReturns
	fake.recordInvocation("Read", []interface{}{arg1Copy})
	fake.readMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *OverwriteableFile) ReadCallCount() int {
	fake.readMutex.RLock()
	defer fake.readMutex.RUnlock()
	return len(fake.readArgsForCall)
}

func (fake *OverwriteableFile) ReadCalls(stub func([]byte) (int, error)) {
	fake.readMutex.Lock()
	defer fake.readMutex.Unlock()
	fake.ReadStub = stub
}

func (fake *OverwriteableFile) ReadArgsForCall(i int) []byte {
	fake.readMutex.RLock()
	defer fake.readMutex.RUnlock()
	argsForCall := fake.readArgsForCall[i]
	return argsForCall.arg1
}

func (fake *OverwriteableFile) ReadReturns(result1 int, result2 error) {
	fake.readMutex.Lock()
	defer fake.readMutex.Unlock()
	fake.ReadStub = nil
	fake.readReturns = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *OverwriteableFile) ReadReturnsOnCall(i int, result1 int, result2 error) {
	fake.readMutex.Lock()
	defer fake.readMutex.Unlock()
	fake.ReadStub = nil
	if fake.readReturnsOnCall == nil {
		fake.readReturnsOnCall = make(map[int]struct {
			result1 int
			result2 error
		})
	}
	fake.readReturnsOnCall[i] = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *OverwriteableFile) Seek(arg1 int64, arg2 int) (int64, error) {
	fake.seekMutex.Lock()
	ret, specificReturn := fake.seekReturnsOnCall[len(fake.seekArgsForCall)]
	fake.seekArgsForCall = append(fake.seekArgsForCall, struct {
		arg1 int64
		arg2 int
	}{arg1, arg2})
	stub := fake.SeekStub
	fakeReturns := fake.seekReturns
	fake.recordInvocation("Seek", []interface{}{arg1, arg2})
	fake.seekMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *OverwriteableFile) SeekCallCount() int {
	fake.seekMutex.RLock()
	defer fake.seekMutex.RUnlock()
	return len(fake.seekArgsForCall)
}

func (fake *OverwriteableFile) SeekCalls(stub func(int64, int) (int64, error)) {
	fake.seekMutex.Lock()
	defer fake.seekMutex.Unlock()
	fake.SeekStub = stub
}

func (fake *OverwriteableFile) SeekArgsForCall(i int) (int64, int) {
	fake.seekMutex.RLock()
	defer fake.seekMutex.RUnlock()
	argsForCall := fake.seekArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *OverwriteableFile) SeekReturns(result1 int64, result2 error) {
	fake.seekMutex.Lock()
	defer fake.seekMutex.Unlock()
	fake.SeekStub = nil
	fake.seekReturns = struct {
		result1 int64
		result2 error
	}{result1, result2}
}

func (fake *OverwriteableFile) SeekReturnsOnCall(i int, result1 int64, result2 error) {
	fake.seekMutex.Lock()
	defer fake.seekMutex.Unlock()
	fake.SeekStub = nil
	if fake.seekReturnsOnCall == nil {
		fake.seekReturnsOnCall = make(map[int]struct {
			result1 int64
			result2 error
		})
	}
	fake.seekReturnsOnCall[i] = struct {
		result1 int64
		result2 error
	}{result1, result2}
}

func (fake *OverwriteableFile) Truncate(arg1 int64) error {
	fake.truncateMutex.Lock()
	ret, specificReturn := fake.truncateReturnsOnCall[len(fake.truncateArgsForCall)]
	fake.truncateArgsForCall = append(fake.truncateArgsForCall, struct {
		arg1 int64
	}{arg1})
	stub := fake.TruncateStub
	fakeReturns := fake.truncateReturns
	fake.recordInvocation("Truncate", []interface{}{arg1})
	fake.truncateMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *OverwriteableFile) TruncateCallCount() int {
	fake.truncateMutex.RLock()
	defer fake.truncateMutex.RUnlock()
	return len(fake.truncateArgsForCall)
}

func (fake *OverwriteableFile) TruncateCalls(stub func(int64) error) {
	fake.truncateMutex.Lock()
	defer fake.truncateMutex.Unlock()
	fake.TruncateStub = stub
}

func (fake *OverwriteableFile) TruncateArgsForCall(i int) int64 {
	fake.truncateMutex.RLock()
	defer fake.truncateMutex.RUnlock()
	argsForCall := fake.truncateArgsForCall[i]
	return argsForCall.arg1
}

func (fake *OverwriteableFile) TruncateReturns(result1 error) {
	fake.truncateMutex.Lock()
	defer fake.truncateMutex.Unlock()
	fake.TruncateStub = nil
	fake.truncateReturns = struct {
		result1 error
	}{result1}
}

func (fake *OverwriteableFile) TruncateReturnsOnCall(i int, result1 error) {
	fake.truncateMutex.Lock()
	defer fake.truncateMutex.Unlock()
	fake.TruncateStub = nil
	if fake.truncateReturnsOnCall == nil {
		fake.truncateReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.truncateReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *OverwriteableFile) Write(arg1 []byte) (int, error) {
	var arg1Copy []byte
	if arg1 != nil {
		arg1Copy = make([]byte, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.writeMutex.Lock()
	ret, specificReturn := fake.writeReturnsOnCall[len(fake.writeArgsForCall)]
	fake.writeArgsForCall = append(fake.writeArgsForCall, struct {
		arg1 []byte
	}{arg1Copy})
	stub := fake.WriteStub
	fakeReturns := fake.writeReturns
	fake.recordInvocation("Write", []interface{}{arg1Copy})
	fake.writeMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *OverwriteableFile) WriteCallCount() int {
	fake.writeMutex.RLock()
	defer fake.writeMutex.RUnlock()
	return len(fake.writeArgsForCall)
}

func (fake *OverwriteableFile) WriteCalls(stub func([]byte) (int, error)) {
	fake.writeMutex.Lock()
	defer fake.writeMutex.Unlock()
	fake.WriteStub = stub
}

func (fake *OverwriteableFile) WriteArgsForCall(i int) []byte {
	fake.writeMutex.RLock()
	defer fake.writeMutex.RUnlock()
	argsForCall := fake.writeArgsForCall[i]
	return argsForCall.arg1
}

func (fake *OverwriteableFile) WriteReturns(result1 int, result2 error) {
	fake.writeMutex.Lock()
	defer fake.writeMutex.Unlock()
	fake.WriteStub = nil
	fake.writeReturns = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *OverwriteableFile) WriteReturnsOnCall(i int, result1 int, result2 error) {
	fake.writeMutex.Lock()
	defer fake.writeMutex.Unlock()
	fake.WriteStub = nil
	if fake.writeReturnsOnCall == nil {
		fake.writeReturnsOnCall = make(map[int]struct {
			result1 int
			result2 error
		})
	}
	fake.writeReturnsOnCall[i] = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *OverwriteableFile) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.readMutex.RLock()
	defer fake.readMutex.RUnlock()
	fake.seekMutex.RLock()
	defer fake.seekMutex.RUnlock()
	fake.truncateMutex.RLock()
	defer fake.truncateMutex.RUnlock()
	fake.writeMutex.RLock()
	defer fake.writeMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *OverwriteableFile) recordInvocation(key string, args []interface{}) {
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

var _ serial.OverwriteableFile = new(OverwriteableFile)
