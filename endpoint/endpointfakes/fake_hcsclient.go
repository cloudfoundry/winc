// Code generated by counterfeiter. DO NOT EDIT.
package endpointfakes

import (
	"sync"

	"code.cloudfoundry.org/winc/endpoint"
	"github.com/Microsoft/hcsshim"
)

type FakeHCSClient struct {
	GetHNSNetworkByNameStub        func(string) (*hcsshim.HNSNetwork, error)
	getHNSNetworkByNameMutex       sync.RWMutex
	getHNSNetworkByNameArgsForCall []struct {
		arg1 string
	}
	getHNSNetworkByNameReturns struct {
		result1 *hcsshim.HNSNetwork
		result2 error
	}
	getHNSNetworkByNameReturnsOnCall map[int]struct {
		result1 *hcsshim.HNSNetwork
		result2 error
	}
	CreateEndpointStub        func(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
	createEndpointMutex       sync.RWMutex
	createEndpointArgsForCall []struct {
		arg1 *hcsshim.HNSEndpoint
	}
	createEndpointReturns struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}
	createEndpointReturnsOnCall map[int]struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}
	GetHNSEndpointByIDStub        func(string) (*hcsshim.HNSEndpoint, error)
	getHNSEndpointByIDMutex       sync.RWMutex
	getHNSEndpointByIDArgsForCall []struct {
		arg1 string
	}
	getHNSEndpointByIDReturns struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}
	getHNSEndpointByIDReturnsOnCall map[int]struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}
	GetHNSEndpointByNameStub        func(string) (*hcsshim.HNSEndpoint, error)
	getHNSEndpointByNameMutex       sync.RWMutex
	getHNSEndpointByNameArgsForCall []struct {
		arg1 string
	}
	getHNSEndpointByNameReturns struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}
	getHNSEndpointByNameReturnsOnCall map[int]struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}
	DeleteEndpointStub        func(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
	deleteEndpointMutex       sync.RWMutex
	deleteEndpointArgsForCall []struct {
		arg1 *hcsshim.HNSEndpoint
	}
	deleteEndpointReturns struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}
	deleteEndpointReturnsOnCall map[int]struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}
	HotAttachEndpointStub        func(containerID string, endpointID string) error
	hotAttachEndpointMutex       sync.RWMutex
	hotAttachEndpointArgsForCall []struct {
		containerID string
		endpointID  string
	}
	hotAttachEndpointReturns struct {
		result1 error
	}
	hotAttachEndpointReturnsOnCall map[int]struct {
		result1 error
	}
	HotDetachEndpointStub        func(containerID string, endpointID string) error
	hotDetachEndpointMutex       sync.RWMutex
	hotDetachEndpointArgsForCall []struct {
		containerID string
		endpointID  string
	}
	hotDetachEndpointReturns struct {
		result1 error
	}
	hotDetachEndpointReturnsOnCall map[int]struct {
		result1 error
	}
	ApplyACLPolicyStub        func(*hcsshim.HNSEndpoint, ...*hcsshim.ACLPolicy) error
	applyACLPolicyMutex       sync.RWMutex
	applyACLPolicyArgsForCall []struct {
		arg1 *hcsshim.HNSEndpoint
		arg2 []*hcsshim.ACLPolicy
	}
	applyACLPolicyReturns struct {
		result1 error
	}
	applyACLPolicyReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeHCSClient) GetHNSNetworkByName(arg1 string) (*hcsshim.HNSNetwork, error) {
	fake.getHNSNetworkByNameMutex.Lock()
	ret, specificReturn := fake.getHNSNetworkByNameReturnsOnCall[len(fake.getHNSNetworkByNameArgsForCall)]
	fake.getHNSNetworkByNameArgsForCall = append(fake.getHNSNetworkByNameArgsForCall, struct {
		arg1 string
	}{arg1})
	fake.recordInvocation("GetHNSNetworkByName", []interface{}{arg1})
	fake.getHNSNetworkByNameMutex.Unlock()
	if fake.GetHNSNetworkByNameStub != nil {
		return fake.GetHNSNetworkByNameStub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.getHNSNetworkByNameReturns.result1, fake.getHNSNetworkByNameReturns.result2
}

func (fake *FakeHCSClient) GetHNSNetworkByNameCallCount() int {
	fake.getHNSNetworkByNameMutex.RLock()
	defer fake.getHNSNetworkByNameMutex.RUnlock()
	return len(fake.getHNSNetworkByNameArgsForCall)
}

func (fake *FakeHCSClient) GetHNSNetworkByNameArgsForCall(i int) string {
	fake.getHNSNetworkByNameMutex.RLock()
	defer fake.getHNSNetworkByNameMutex.RUnlock()
	return fake.getHNSNetworkByNameArgsForCall[i].arg1
}

func (fake *FakeHCSClient) GetHNSNetworkByNameReturns(result1 *hcsshim.HNSNetwork, result2 error) {
	fake.GetHNSNetworkByNameStub = nil
	fake.getHNSNetworkByNameReturns = struct {
		result1 *hcsshim.HNSNetwork
		result2 error
	}{result1, result2}
}

func (fake *FakeHCSClient) GetHNSNetworkByNameReturnsOnCall(i int, result1 *hcsshim.HNSNetwork, result2 error) {
	fake.GetHNSNetworkByNameStub = nil
	if fake.getHNSNetworkByNameReturnsOnCall == nil {
		fake.getHNSNetworkByNameReturnsOnCall = make(map[int]struct {
			result1 *hcsshim.HNSNetwork
			result2 error
		})
	}
	fake.getHNSNetworkByNameReturnsOnCall[i] = struct {
		result1 *hcsshim.HNSNetwork
		result2 error
	}{result1, result2}
}

func (fake *FakeHCSClient) CreateEndpoint(arg1 *hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error) {
	fake.createEndpointMutex.Lock()
	ret, specificReturn := fake.createEndpointReturnsOnCall[len(fake.createEndpointArgsForCall)]
	fake.createEndpointArgsForCall = append(fake.createEndpointArgsForCall, struct {
		arg1 *hcsshim.HNSEndpoint
	}{arg1})
	fake.recordInvocation("CreateEndpoint", []interface{}{arg1})
	fake.createEndpointMutex.Unlock()
	if fake.CreateEndpointStub != nil {
		return fake.CreateEndpointStub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.createEndpointReturns.result1, fake.createEndpointReturns.result2
}

func (fake *FakeHCSClient) CreateEndpointCallCount() int {
	fake.createEndpointMutex.RLock()
	defer fake.createEndpointMutex.RUnlock()
	return len(fake.createEndpointArgsForCall)
}

func (fake *FakeHCSClient) CreateEndpointArgsForCall(i int) *hcsshim.HNSEndpoint {
	fake.createEndpointMutex.RLock()
	defer fake.createEndpointMutex.RUnlock()
	return fake.createEndpointArgsForCall[i].arg1
}

func (fake *FakeHCSClient) CreateEndpointReturns(result1 *hcsshim.HNSEndpoint, result2 error) {
	fake.CreateEndpointStub = nil
	fake.createEndpointReturns = struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *FakeHCSClient) CreateEndpointReturnsOnCall(i int, result1 *hcsshim.HNSEndpoint, result2 error) {
	fake.CreateEndpointStub = nil
	if fake.createEndpointReturnsOnCall == nil {
		fake.createEndpointReturnsOnCall = make(map[int]struct {
			result1 *hcsshim.HNSEndpoint
			result2 error
		})
	}
	fake.createEndpointReturnsOnCall[i] = struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *FakeHCSClient) GetHNSEndpointByID(arg1 string) (*hcsshim.HNSEndpoint, error) {
	fake.getHNSEndpointByIDMutex.Lock()
	ret, specificReturn := fake.getHNSEndpointByIDReturnsOnCall[len(fake.getHNSEndpointByIDArgsForCall)]
	fake.getHNSEndpointByIDArgsForCall = append(fake.getHNSEndpointByIDArgsForCall, struct {
		arg1 string
	}{arg1})
	fake.recordInvocation("GetHNSEndpointByID", []interface{}{arg1})
	fake.getHNSEndpointByIDMutex.Unlock()
	if fake.GetHNSEndpointByIDStub != nil {
		return fake.GetHNSEndpointByIDStub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.getHNSEndpointByIDReturns.result1, fake.getHNSEndpointByIDReturns.result2
}

func (fake *FakeHCSClient) GetHNSEndpointByIDCallCount() int {
	fake.getHNSEndpointByIDMutex.RLock()
	defer fake.getHNSEndpointByIDMutex.RUnlock()
	return len(fake.getHNSEndpointByIDArgsForCall)
}

func (fake *FakeHCSClient) GetHNSEndpointByIDArgsForCall(i int) string {
	fake.getHNSEndpointByIDMutex.RLock()
	defer fake.getHNSEndpointByIDMutex.RUnlock()
	return fake.getHNSEndpointByIDArgsForCall[i].arg1
}

func (fake *FakeHCSClient) GetHNSEndpointByIDReturns(result1 *hcsshim.HNSEndpoint, result2 error) {
	fake.GetHNSEndpointByIDStub = nil
	fake.getHNSEndpointByIDReturns = struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *FakeHCSClient) GetHNSEndpointByIDReturnsOnCall(i int, result1 *hcsshim.HNSEndpoint, result2 error) {
	fake.GetHNSEndpointByIDStub = nil
	if fake.getHNSEndpointByIDReturnsOnCall == nil {
		fake.getHNSEndpointByIDReturnsOnCall = make(map[int]struct {
			result1 *hcsshim.HNSEndpoint
			result2 error
		})
	}
	fake.getHNSEndpointByIDReturnsOnCall[i] = struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *FakeHCSClient) GetHNSEndpointByName(arg1 string) (*hcsshim.HNSEndpoint, error) {
	fake.getHNSEndpointByNameMutex.Lock()
	ret, specificReturn := fake.getHNSEndpointByNameReturnsOnCall[len(fake.getHNSEndpointByNameArgsForCall)]
	fake.getHNSEndpointByNameArgsForCall = append(fake.getHNSEndpointByNameArgsForCall, struct {
		arg1 string
	}{arg1})
	fake.recordInvocation("GetHNSEndpointByName", []interface{}{arg1})
	fake.getHNSEndpointByNameMutex.Unlock()
	if fake.GetHNSEndpointByNameStub != nil {
		return fake.GetHNSEndpointByNameStub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.getHNSEndpointByNameReturns.result1, fake.getHNSEndpointByNameReturns.result2
}

func (fake *FakeHCSClient) GetHNSEndpointByNameCallCount() int {
	fake.getHNSEndpointByNameMutex.RLock()
	defer fake.getHNSEndpointByNameMutex.RUnlock()
	return len(fake.getHNSEndpointByNameArgsForCall)
}

func (fake *FakeHCSClient) GetHNSEndpointByNameArgsForCall(i int) string {
	fake.getHNSEndpointByNameMutex.RLock()
	defer fake.getHNSEndpointByNameMutex.RUnlock()
	return fake.getHNSEndpointByNameArgsForCall[i].arg1
}

func (fake *FakeHCSClient) GetHNSEndpointByNameReturns(result1 *hcsshim.HNSEndpoint, result2 error) {
	fake.GetHNSEndpointByNameStub = nil
	fake.getHNSEndpointByNameReturns = struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *FakeHCSClient) GetHNSEndpointByNameReturnsOnCall(i int, result1 *hcsshim.HNSEndpoint, result2 error) {
	fake.GetHNSEndpointByNameStub = nil
	if fake.getHNSEndpointByNameReturnsOnCall == nil {
		fake.getHNSEndpointByNameReturnsOnCall = make(map[int]struct {
			result1 *hcsshim.HNSEndpoint
			result2 error
		})
	}
	fake.getHNSEndpointByNameReturnsOnCall[i] = struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *FakeHCSClient) DeleteEndpoint(arg1 *hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error) {
	fake.deleteEndpointMutex.Lock()
	ret, specificReturn := fake.deleteEndpointReturnsOnCall[len(fake.deleteEndpointArgsForCall)]
	fake.deleteEndpointArgsForCall = append(fake.deleteEndpointArgsForCall, struct {
		arg1 *hcsshim.HNSEndpoint
	}{arg1})
	fake.recordInvocation("DeleteEndpoint", []interface{}{arg1})
	fake.deleteEndpointMutex.Unlock()
	if fake.DeleteEndpointStub != nil {
		return fake.DeleteEndpointStub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.deleteEndpointReturns.result1, fake.deleteEndpointReturns.result2
}

func (fake *FakeHCSClient) DeleteEndpointCallCount() int {
	fake.deleteEndpointMutex.RLock()
	defer fake.deleteEndpointMutex.RUnlock()
	return len(fake.deleteEndpointArgsForCall)
}

func (fake *FakeHCSClient) DeleteEndpointArgsForCall(i int) *hcsshim.HNSEndpoint {
	fake.deleteEndpointMutex.RLock()
	defer fake.deleteEndpointMutex.RUnlock()
	return fake.deleteEndpointArgsForCall[i].arg1
}

func (fake *FakeHCSClient) DeleteEndpointReturns(result1 *hcsshim.HNSEndpoint, result2 error) {
	fake.DeleteEndpointStub = nil
	fake.deleteEndpointReturns = struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *FakeHCSClient) DeleteEndpointReturnsOnCall(i int, result1 *hcsshim.HNSEndpoint, result2 error) {
	fake.DeleteEndpointStub = nil
	if fake.deleteEndpointReturnsOnCall == nil {
		fake.deleteEndpointReturnsOnCall = make(map[int]struct {
			result1 *hcsshim.HNSEndpoint
			result2 error
		})
	}
	fake.deleteEndpointReturnsOnCall[i] = struct {
		result1 *hcsshim.HNSEndpoint
		result2 error
	}{result1, result2}
}

func (fake *FakeHCSClient) HotAttachEndpoint(containerID string, endpointID string) error {
	fake.hotAttachEndpointMutex.Lock()
	ret, specificReturn := fake.hotAttachEndpointReturnsOnCall[len(fake.hotAttachEndpointArgsForCall)]
	fake.hotAttachEndpointArgsForCall = append(fake.hotAttachEndpointArgsForCall, struct {
		containerID string
		endpointID  string
	}{containerID, endpointID})
	fake.recordInvocation("HotAttachEndpoint", []interface{}{containerID, endpointID})
	fake.hotAttachEndpointMutex.Unlock()
	if fake.HotAttachEndpointStub != nil {
		return fake.HotAttachEndpointStub(containerID, endpointID)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.hotAttachEndpointReturns.result1
}

func (fake *FakeHCSClient) HotAttachEndpointCallCount() int {
	fake.hotAttachEndpointMutex.RLock()
	defer fake.hotAttachEndpointMutex.RUnlock()
	return len(fake.hotAttachEndpointArgsForCall)
}

func (fake *FakeHCSClient) HotAttachEndpointArgsForCall(i int) (string, string) {
	fake.hotAttachEndpointMutex.RLock()
	defer fake.hotAttachEndpointMutex.RUnlock()
	return fake.hotAttachEndpointArgsForCall[i].containerID, fake.hotAttachEndpointArgsForCall[i].endpointID
}

func (fake *FakeHCSClient) HotAttachEndpointReturns(result1 error) {
	fake.HotAttachEndpointStub = nil
	fake.hotAttachEndpointReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeHCSClient) HotAttachEndpointReturnsOnCall(i int, result1 error) {
	fake.HotAttachEndpointStub = nil
	if fake.hotAttachEndpointReturnsOnCall == nil {
		fake.hotAttachEndpointReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.hotAttachEndpointReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeHCSClient) HotDetachEndpoint(containerID string, endpointID string) error {
	fake.hotDetachEndpointMutex.Lock()
	ret, specificReturn := fake.hotDetachEndpointReturnsOnCall[len(fake.hotDetachEndpointArgsForCall)]
	fake.hotDetachEndpointArgsForCall = append(fake.hotDetachEndpointArgsForCall, struct {
		containerID string
		endpointID  string
	}{containerID, endpointID})
	fake.recordInvocation("HotDetachEndpoint", []interface{}{containerID, endpointID})
	fake.hotDetachEndpointMutex.Unlock()
	if fake.HotDetachEndpointStub != nil {
		return fake.HotDetachEndpointStub(containerID, endpointID)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.hotDetachEndpointReturns.result1
}

func (fake *FakeHCSClient) HotDetachEndpointCallCount() int {
	fake.hotDetachEndpointMutex.RLock()
	defer fake.hotDetachEndpointMutex.RUnlock()
	return len(fake.hotDetachEndpointArgsForCall)
}

func (fake *FakeHCSClient) HotDetachEndpointArgsForCall(i int) (string, string) {
	fake.hotDetachEndpointMutex.RLock()
	defer fake.hotDetachEndpointMutex.RUnlock()
	return fake.hotDetachEndpointArgsForCall[i].containerID, fake.hotDetachEndpointArgsForCall[i].endpointID
}

func (fake *FakeHCSClient) HotDetachEndpointReturns(result1 error) {
	fake.HotDetachEndpointStub = nil
	fake.hotDetachEndpointReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeHCSClient) HotDetachEndpointReturnsOnCall(i int, result1 error) {
	fake.HotDetachEndpointStub = nil
	if fake.hotDetachEndpointReturnsOnCall == nil {
		fake.hotDetachEndpointReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.hotDetachEndpointReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeHCSClient) ApplyACLPolicy(arg1 *hcsshim.HNSEndpoint, arg2 ...*hcsshim.ACLPolicy) error {
	fake.applyACLPolicyMutex.Lock()
	ret, specificReturn := fake.applyACLPolicyReturnsOnCall[len(fake.applyACLPolicyArgsForCall)]
	fake.applyACLPolicyArgsForCall = append(fake.applyACLPolicyArgsForCall, struct {
		arg1 *hcsshim.HNSEndpoint
		arg2 []*hcsshim.ACLPolicy
	}{arg1, arg2})
	fake.recordInvocation("ApplyACLPolicy", []interface{}{arg1, arg2})
	fake.applyACLPolicyMutex.Unlock()
	if fake.ApplyACLPolicyStub != nil {
		return fake.ApplyACLPolicyStub(arg1, arg2...)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.applyACLPolicyReturns.result1
}

func (fake *FakeHCSClient) ApplyACLPolicyCallCount() int {
	fake.applyACLPolicyMutex.RLock()
	defer fake.applyACLPolicyMutex.RUnlock()
	return len(fake.applyACLPolicyArgsForCall)
}

func (fake *FakeHCSClient) ApplyACLPolicyArgsForCall(i int) (*hcsshim.HNSEndpoint, []*hcsshim.ACLPolicy) {
	fake.applyACLPolicyMutex.RLock()
	defer fake.applyACLPolicyMutex.RUnlock()
	return fake.applyACLPolicyArgsForCall[i].arg1, fake.applyACLPolicyArgsForCall[i].arg2
}

func (fake *FakeHCSClient) ApplyACLPolicyReturns(result1 error) {
	fake.ApplyACLPolicyStub = nil
	fake.applyACLPolicyReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeHCSClient) ApplyACLPolicyReturnsOnCall(i int, result1 error) {
	fake.ApplyACLPolicyStub = nil
	if fake.applyACLPolicyReturnsOnCall == nil {
		fake.applyACLPolicyReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.applyACLPolicyReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeHCSClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getHNSNetworkByNameMutex.RLock()
	defer fake.getHNSNetworkByNameMutex.RUnlock()
	fake.createEndpointMutex.RLock()
	defer fake.createEndpointMutex.RUnlock()
	fake.getHNSEndpointByIDMutex.RLock()
	defer fake.getHNSEndpointByIDMutex.RUnlock()
	fake.getHNSEndpointByNameMutex.RLock()
	defer fake.getHNSEndpointByNameMutex.RUnlock()
	fake.deleteEndpointMutex.RLock()
	defer fake.deleteEndpointMutex.RUnlock()
	fake.hotAttachEndpointMutex.RLock()
	defer fake.hotAttachEndpointMutex.RUnlock()
	fake.hotDetachEndpointMutex.RLock()
	defer fake.hotDetachEndpointMutex.RUnlock()
	fake.applyACLPolicyMutex.RLock()
	defer fake.applyACLPolicyMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeHCSClient) recordInvocation(key string, args []interface{}) {
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

var _ endpoint.HCSClient = new(FakeHCSClient)
