// This file was generated by counterfeiter
package hcsclientfakes

import (
	"sync"

	"code.cloudfoundry.org/winc/hcsclient"
	"github.com/Microsoft/hcsshim"
)

type FakeClient struct {
	GetContainersStub        func(q hcsshim.ComputeSystemQuery) ([]hcsshim.ContainerProperties, error)
	getContainersMutex       sync.RWMutex
	getContainersArgsForCall []struct {
		q hcsshim.ComputeSystemQuery
	}
	getContainersReturns struct {
		result1 []hcsshim.ContainerProperties
		result2 error
	}
	NameToGuidStub        func(name string) (hcsshim.GUID, error)
	nameToGuidMutex       sync.RWMutex
	nameToGuidArgsForCall []struct {
		name string
	}
	nameToGuidReturns struct {
		result1 hcsshim.GUID
		result2 error
	}
	GetLayerMountPathStub        func(info hcsshim.DriverInfo, id string) (string, error)
	getLayerMountPathMutex       sync.RWMutex
	getLayerMountPathArgsForCall []struct {
		info hcsshim.DriverInfo
		id   string
	}
	getLayerMountPathReturns struct {
		result1 string
		result2 error
	}
	CreateContainerStub        func(id string, config *hcsshim.ContainerConfig) (hcsshim.Container, error)
	createContainerMutex       sync.RWMutex
	createContainerArgsForCall []struct {
		id     string
		config *hcsshim.ContainerConfig
	}
	createContainerReturns struct {
		result1 hcsshim.Container
		result2 error
	}
	OpenContainerStub        func(id string) (hcsshim.Container, error)
	openContainerMutex       sync.RWMutex
	openContainerArgsForCall []struct {
		id string
	}
	openContainerReturns struct {
		result1 hcsshim.Container
		result2 error
	}
	IsPendingStub        func(err error) bool
	isPendingMutex       sync.RWMutex
	isPendingArgsForCall []struct {
		err error
	}
	isPendingReturns struct {
		result1 bool
	}
	CreateSandboxLayerStub        func(info hcsshim.DriverInfo, layerId, parentId string, parentLayerPaths []string) error
	createSandboxLayerMutex       sync.RWMutex
	createSandboxLayerArgsForCall []struct {
		info             hcsshim.DriverInfo
		layerId          string
		parentId         string
		parentLayerPaths []string
	}
	createSandboxLayerReturns struct {
		result1 error
	}
	ActivateLayerStub        func(info hcsshim.DriverInfo, id string) error
	activateLayerMutex       sync.RWMutex
	activateLayerArgsForCall []struct {
		info hcsshim.DriverInfo
		id   string
	}
	activateLayerReturns struct {
		result1 error
	}
	PrepareLayerStub        func(info hcsshim.DriverInfo, layerId string, parentLayerPaths []string) error
	prepareLayerMutex       sync.RWMutex
	prepareLayerArgsForCall []struct {
		info             hcsshim.DriverInfo
		layerId          string
		parentLayerPaths []string
	}
	prepareLayerReturns struct {
		result1 error
	}
	UnprepareLayerStub        func(info hcsshim.DriverInfo, layerId string) error
	unprepareLayerMutex       sync.RWMutex
	unprepareLayerArgsForCall []struct {
		info    hcsshim.DriverInfo
		layerId string
	}
	unprepareLayerReturns struct {
		result1 error
	}
	DeactivateLayerStub        func(info hcsshim.DriverInfo, id string) error
	deactivateLayerMutex       sync.RWMutex
	deactivateLayerArgsForCall []struct {
		info hcsshim.DriverInfo
		id   string
	}
	deactivateLayerReturns struct {
		result1 error
	}
	DestroyLayerStub        func(info hcsshim.DriverInfo, id string) error
	destroyLayerMutex       sync.RWMutex
	destroyLayerArgsForCall []struct {
		info hcsshim.DriverInfo
		id   string
	}
	destroyLayerReturns struct {
		result1 error
	}
	GetContainerPropertiesStub        func(id string) (hcsshim.ContainerProperties, error)
	getContainerPropertiesMutex       sync.RWMutex
	getContainerPropertiesArgsForCall []struct {
		id string
	}
	getContainerPropertiesReturns struct {
		result1 hcsshim.ContainerProperties
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeClient) GetContainers(q hcsshim.ComputeSystemQuery) ([]hcsshim.ContainerProperties, error) {
	fake.getContainersMutex.Lock()
	fake.getContainersArgsForCall = append(fake.getContainersArgsForCall, struct {
		q hcsshim.ComputeSystemQuery
	}{q})
	fake.recordInvocation("GetContainers", []interface{}{q})
	fake.getContainersMutex.Unlock()
	if fake.GetContainersStub != nil {
		return fake.GetContainersStub(q)
	} else {
		return fake.getContainersReturns.result1, fake.getContainersReturns.result2
	}
}

func (fake *FakeClient) GetContainersCallCount() int {
	fake.getContainersMutex.RLock()
	defer fake.getContainersMutex.RUnlock()
	return len(fake.getContainersArgsForCall)
}

func (fake *FakeClient) GetContainersArgsForCall(i int) hcsshim.ComputeSystemQuery {
	fake.getContainersMutex.RLock()
	defer fake.getContainersMutex.RUnlock()
	return fake.getContainersArgsForCall[i].q
}

func (fake *FakeClient) GetContainersReturns(result1 []hcsshim.ContainerProperties, result2 error) {
	fake.GetContainersStub = nil
	fake.getContainersReturns = struct {
		result1 []hcsshim.ContainerProperties
		result2 error
	}{result1, result2}
}

func (fake *FakeClient) NameToGuid(name string) (hcsshim.GUID, error) {
	fake.nameToGuidMutex.Lock()
	fake.nameToGuidArgsForCall = append(fake.nameToGuidArgsForCall, struct {
		name string
	}{name})
	fake.recordInvocation("NameToGuid", []interface{}{name})
	fake.nameToGuidMutex.Unlock()
	if fake.NameToGuidStub != nil {
		return fake.NameToGuidStub(name)
	} else {
		return fake.nameToGuidReturns.result1, fake.nameToGuidReturns.result2
	}
}

func (fake *FakeClient) NameToGuidCallCount() int {
	fake.nameToGuidMutex.RLock()
	defer fake.nameToGuidMutex.RUnlock()
	return len(fake.nameToGuidArgsForCall)
}

func (fake *FakeClient) NameToGuidArgsForCall(i int) string {
	fake.nameToGuidMutex.RLock()
	defer fake.nameToGuidMutex.RUnlock()
	return fake.nameToGuidArgsForCall[i].name
}

func (fake *FakeClient) NameToGuidReturns(result1 hcsshim.GUID, result2 error) {
	fake.NameToGuidStub = nil
	fake.nameToGuidReturns = struct {
		result1 hcsshim.GUID
		result2 error
	}{result1, result2}
}

func (fake *FakeClient) GetLayerMountPath(info hcsshim.DriverInfo, id string) (string, error) {
	fake.getLayerMountPathMutex.Lock()
	fake.getLayerMountPathArgsForCall = append(fake.getLayerMountPathArgsForCall, struct {
		info hcsshim.DriverInfo
		id   string
	}{info, id})
	fake.recordInvocation("GetLayerMountPath", []interface{}{info, id})
	fake.getLayerMountPathMutex.Unlock()
	if fake.GetLayerMountPathStub != nil {
		return fake.GetLayerMountPathStub(info, id)
	} else {
		return fake.getLayerMountPathReturns.result1, fake.getLayerMountPathReturns.result2
	}
}

func (fake *FakeClient) GetLayerMountPathCallCount() int {
	fake.getLayerMountPathMutex.RLock()
	defer fake.getLayerMountPathMutex.RUnlock()
	return len(fake.getLayerMountPathArgsForCall)
}

func (fake *FakeClient) GetLayerMountPathArgsForCall(i int) (hcsshim.DriverInfo, string) {
	fake.getLayerMountPathMutex.RLock()
	defer fake.getLayerMountPathMutex.RUnlock()
	return fake.getLayerMountPathArgsForCall[i].info, fake.getLayerMountPathArgsForCall[i].id
}

func (fake *FakeClient) GetLayerMountPathReturns(result1 string, result2 error) {
	fake.GetLayerMountPathStub = nil
	fake.getLayerMountPathReturns = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakeClient) CreateContainer(id string, config *hcsshim.ContainerConfig) (hcsshim.Container, error) {
	fake.createContainerMutex.Lock()
	fake.createContainerArgsForCall = append(fake.createContainerArgsForCall, struct {
		id     string
		config *hcsshim.ContainerConfig
	}{id, config})
	fake.recordInvocation("CreateContainer", []interface{}{id, config})
	fake.createContainerMutex.Unlock()
	if fake.CreateContainerStub != nil {
		return fake.CreateContainerStub(id, config)
	} else {
		return fake.createContainerReturns.result1, fake.createContainerReturns.result2
	}
}

func (fake *FakeClient) CreateContainerCallCount() int {
	fake.createContainerMutex.RLock()
	defer fake.createContainerMutex.RUnlock()
	return len(fake.createContainerArgsForCall)
}

func (fake *FakeClient) CreateContainerArgsForCall(i int) (string, *hcsshim.ContainerConfig) {
	fake.createContainerMutex.RLock()
	defer fake.createContainerMutex.RUnlock()
	return fake.createContainerArgsForCall[i].id, fake.createContainerArgsForCall[i].config
}

func (fake *FakeClient) CreateContainerReturns(result1 hcsshim.Container, result2 error) {
	fake.CreateContainerStub = nil
	fake.createContainerReturns = struct {
		result1 hcsshim.Container
		result2 error
	}{result1, result2}
}

func (fake *FakeClient) OpenContainer(id string) (hcsshim.Container, error) {
	fake.openContainerMutex.Lock()
	fake.openContainerArgsForCall = append(fake.openContainerArgsForCall, struct {
		id string
	}{id})
	fake.recordInvocation("OpenContainer", []interface{}{id})
	fake.openContainerMutex.Unlock()
	if fake.OpenContainerStub != nil {
		return fake.OpenContainerStub(id)
	} else {
		return fake.openContainerReturns.result1, fake.openContainerReturns.result2
	}
}

func (fake *FakeClient) OpenContainerCallCount() int {
	fake.openContainerMutex.RLock()
	defer fake.openContainerMutex.RUnlock()
	return len(fake.openContainerArgsForCall)
}

func (fake *FakeClient) OpenContainerArgsForCall(i int) string {
	fake.openContainerMutex.RLock()
	defer fake.openContainerMutex.RUnlock()
	return fake.openContainerArgsForCall[i].id
}

func (fake *FakeClient) OpenContainerReturns(result1 hcsshim.Container, result2 error) {
	fake.OpenContainerStub = nil
	fake.openContainerReturns = struct {
		result1 hcsshim.Container
		result2 error
	}{result1, result2}
}

func (fake *FakeClient) IsPending(err error) bool {
	fake.isPendingMutex.Lock()
	fake.isPendingArgsForCall = append(fake.isPendingArgsForCall, struct {
		err error
	}{err})
	fake.recordInvocation("IsPending", []interface{}{err})
	fake.isPendingMutex.Unlock()
	if fake.IsPendingStub != nil {
		return fake.IsPendingStub(err)
	} else {
		return fake.isPendingReturns.result1
	}
}

func (fake *FakeClient) IsPendingCallCount() int {
	fake.isPendingMutex.RLock()
	defer fake.isPendingMutex.RUnlock()
	return len(fake.isPendingArgsForCall)
}

func (fake *FakeClient) IsPendingArgsForCall(i int) error {
	fake.isPendingMutex.RLock()
	defer fake.isPendingMutex.RUnlock()
	return fake.isPendingArgsForCall[i].err
}

func (fake *FakeClient) IsPendingReturns(result1 bool) {
	fake.IsPendingStub = nil
	fake.isPendingReturns = struct {
		result1 bool
	}{result1}
}

func (fake *FakeClient) CreateSandboxLayer(info hcsshim.DriverInfo, layerId string, parentId string, parentLayerPaths []string) error {
	var parentLayerPathsCopy []string
	if parentLayerPaths != nil {
		parentLayerPathsCopy = make([]string, len(parentLayerPaths))
		copy(parentLayerPathsCopy, parentLayerPaths)
	}
	fake.createSandboxLayerMutex.Lock()
	fake.createSandboxLayerArgsForCall = append(fake.createSandboxLayerArgsForCall, struct {
		info             hcsshim.DriverInfo
		layerId          string
		parentId         string
		parentLayerPaths []string
	}{info, layerId, parentId, parentLayerPathsCopy})
	fake.recordInvocation("CreateSandboxLayer", []interface{}{info, layerId, parentId, parentLayerPathsCopy})
	fake.createSandboxLayerMutex.Unlock()
	if fake.CreateSandboxLayerStub != nil {
		return fake.CreateSandboxLayerStub(info, layerId, parentId, parentLayerPaths)
	} else {
		return fake.createSandboxLayerReturns.result1
	}
}

func (fake *FakeClient) CreateSandboxLayerCallCount() int {
	fake.createSandboxLayerMutex.RLock()
	defer fake.createSandboxLayerMutex.RUnlock()
	return len(fake.createSandboxLayerArgsForCall)
}

func (fake *FakeClient) CreateSandboxLayerArgsForCall(i int) (hcsshim.DriverInfo, string, string, []string) {
	fake.createSandboxLayerMutex.RLock()
	defer fake.createSandboxLayerMutex.RUnlock()
	return fake.createSandboxLayerArgsForCall[i].info, fake.createSandboxLayerArgsForCall[i].layerId, fake.createSandboxLayerArgsForCall[i].parentId, fake.createSandboxLayerArgsForCall[i].parentLayerPaths
}

func (fake *FakeClient) CreateSandboxLayerReturns(result1 error) {
	fake.CreateSandboxLayerStub = nil
	fake.createSandboxLayerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClient) ActivateLayer(info hcsshim.DriverInfo, id string) error {
	fake.activateLayerMutex.Lock()
	fake.activateLayerArgsForCall = append(fake.activateLayerArgsForCall, struct {
		info hcsshim.DriverInfo
		id   string
	}{info, id})
	fake.recordInvocation("ActivateLayer", []interface{}{info, id})
	fake.activateLayerMutex.Unlock()
	if fake.ActivateLayerStub != nil {
		return fake.ActivateLayerStub(info, id)
	} else {
		return fake.activateLayerReturns.result1
	}
}

func (fake *FakeClient) ActivateLayerCallCount() int {
	fake.activateLayerMutex.RLock()
	defer fake.activateLayerMutex.RUnlock()
	return len(fake.activateLayerArgsForCall)
}

func (fake *FakeClient) ActivateLayerArgsForCall(i int) (hcsshim.DriverInfo, string) {
	fake.activateLayerMutex.RLock()
	defer fake.activateLayerMutex.RUnlock()
	return fake.activateLayerArgsForCall[i].info, fake.activateLayerArgsForCall[i].id
}

func (fake *FakeClient) ActivateLayerReturns(result1 error) {
	fake.ActivateLayerStub = nil
	fake.activateLayerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClient) PrepareLayer(info hcsshim.DriverInfo, layerId string, parentLayerPaths []string) error {
	var parentLayerPathsCopy []string
	if parentLayerPaths != nil {
		parentLayerPathsCopy = make([]string, len(parentLayerPaths))
		copy(parentLayerPathsCopy, parentLayerPaths)
	}
	fake.prepareLayerMutex.Lock()
	fake.prepareLayerArgsForCall = append(fake.prepareLayerArgsForCall, struct {
		info             hcsshim.DriverInfo
		layerId          string
		parentLayerPaths []string
	}{info, layerId, parentLayerPathsCopy})
	fake.recordInvocation("PrepareLayer", []interface{}{info, layerId, parentLayerPathsCopy})
	fake.prepareLayerMutex.Unlock()
	if fake.PrepareLayerStub != nil {
		return fake.PrepareLayerStub(info, layerId, parentLayerPaths)
	} else {
		return fake.prepareLayerReturns.result1
	}
}

func (fake *FakeClient) PrepareLayerCallCount() int {
	fake.prepareLayerMutex.RLock()
	defer fake.prepareLayerMutex.RUnlock()
	return len(fake.prepareLayerArgsForCall)
}

func (fake *FakeClient) PrepareLayerArgsForCall(i int) (hcsshim.DriverInfo, string, []string) {
	fake.prepareLayerMutex.RLock()
	defer fake.prepareLayerMutex.RUnlock()
	return fake.prepareLayerArgsForCall[i].info, fake.prepareLayerArgsForCall[i].layerId, fake.prepareLayerArgsForCall[i].parentLayerPaths
}

func (fake *FakeClient) PrepareLayerReturns(result1 error) {
	fake.PrepareLayerStub = nil
	fake.prepareLayerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClient) UnprepareLayer(info hcsshim.DriverInfo, layerId string) error {
	fake.unprepareLayerMutex.Lock()
	fake.unprepareLayerArgsForCall = append(fake.unprepareLayerArgsForCall, struct {
		info    hcsshim.DriverInfo
		layerId string
	}{info, layerId})
	fake.recordInvocation("UnprepareLayer", []interface{}{info, layerId})
	fake.unprepareLayerMutex.Unlock()
	if fake.UnprepareLayerStub != nil {
		return fake.UnprepareLayerStub(info, layerId)
	} else {
		return fake.unprepareLayerReturns.result1
	}
}

func (fake *FakeClient) UnprepareLayerCallCount() int {
	fake.unprepareLayerMutex.RLock()
	defer fake.unprepareLayerMutex.RUnlock()
	return len(fake.unprepareLayerArgsForCall)
}

func (fake *FakeClient) UnprepareLayerArgsForCall(i int) (hcsshim.DriverInfo, string) {
	fake.unprepareLayerMutex.RLock()
	defer fake.unprepareLayerMutex.RUnlock()
	return fake.unprepareLayerArgsForCall[i].info, fake.unprepareLayerArgsForCall[i].layerId
}

func (fake *FakeClient) UnprepareLayerReturns(result1 error) {
	fake.UnprepareLayerStub = nil
	fake.unprepareLayerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClient) DeactivateLayer(info hcsshim.DriverInfo, id string) error {
	fake.deactivateLayerMutex.Lock()
	fake.deactivateLayerArgsForCall = append(fake.deactivateLayerArgsForCall, struct {
		info hcsshim.DriverInfo
		id   string
	}{info, id})
	fake.recordInvocation("DeactivateLayer", []interface{}{info, id})
	fake.deactivateLayerMutex.Unlock()
	if fake.DeactivateLayerStub != nil {
		return fake.DeactivateLayerStub(info, id)
	} else {
		return fake.deactivateLayerReturns.result1
	}
}

func (fake *FakeClient) DeactivateLayerCallCount() int {
	fake.deactivateLayerMutex.RLock()
	defer fake.deactivateLayerMutex.RUnlock()
	return len(fake.deactivateLayerArgsForCall)
}

func (fake *FakeClient) DeactivateLayerArgsForCall(i int) (hcsshim.DriverInfo, string) {
	fake.deactivateLayerMutex.RLock()
	defer fake.deactivateLayerMutex.RUnlock()
	return fake.deactivateLayerArgsForCall[i].info, fake.deactivateLayerArgsForCall[i].id
}

func (fake *FakeClient) DeactivateLayerReturns(result1 error) {
	fake.DeactivateLayerStub = nil
	fake.deactivateLayerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClient) DestroyLayer(info hcsshim.DriverInfo, id string) error {
	fake.destroyLayerMutex.Lock()
	fake.destroyLayerArgsForCall = append(fake.destroyLayerArgsForCall, struct {
		info hcsshim.DriverInfo
		id   string
	}{info, id})
	fake.recordInvocation("DestroyLayer", []interface{}{info, id})
	fake.destroyLayerMutex.Unlock()
	if fake.DestroyLayerStub != nil {
		return fake.DestroyLayerStub(info, id)
	} else {
		return fake.destroyLayerReturns.result1
	}
}

func (fake *FakeClient) DestroyLayerCallCount() int {
	fake.destroyLayerMutex.RLock()
	defer fake.destroyLayerMutex.RUnlock()
	return len(fake.destroyLayerArgsForCall)
}

func (fake *FakeClient) DestroyLayerArgsForCall(i int) (hcsshim.DriverInfo, string) {
	fake.destroyLayerMutex.RLock()
	defer fake.destroyLayerMutex.RUnlock()
	return fake.destroyLayerArgsForCall[i].info, fake.destroyLayerArgsForCall[i].id
}

func (fake *FakeClient) DestroyLayerReturns(result1 error) {
	fake.DestroyLayerStub = nil
	fake.destroyLayerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClient) GetContainerProperties(id string) (hcsshim.ContainerProperties, error) {
	fake.getContainerPropertiesMutex.Lock()
	fake.getContainerPropertiesArgsForCall = append(fake.getContainerPropertiesArgsForCall, struct {
		id string
	}{id})
	fake.recordInvocation("GetContainerProperties", []interface{}{id})
	fake.getContainerPropertiesMutex.Unlock()
	if fake.GetContainerPropertiesStub != nil {
		return fake.GetContainerPropertiesStub(id)
	} else {
		return fake.getContainerPropertiesReturns.result1, fake.getContainerPropertiesReturns.result2
	}
}

func (fake *FakeClient) GetContainerPropertiesCallCount() int {
	fake.getContainerPropertiesMutex.RLock()
	defer fake.getContainerPropertiesMutex.RUnlock()
	return len(fake.getContainerPropertiesArgsForCall)
}

func (fake *FakeClient) GetContainerPropertiesArgsForCall(i int) string {
	fake.getContainerPropertiesMutex.RLock()
	defer fake.getContainerPropertiesMutex.RUnlock()
	return fake.getContainerPropertiesArgsForCall[i].id
}

func (fake *FakeClient) GetContainerPropertiesReturns(result1 hcsshim.ContainerProperties, result2 error) {
	fake.GetContainerPropertiesStub = nil
	fake.getContainerPropertiesReturns = struct {
		result1 hcsshim.ContainerProperties
		result2 error
	}{result1, result2}
}

func (fake *FakeClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getContainersMutex.RLock()
	defer fake.getContainersMutex.RUnlock()
	fake.nameToGuidMutex.RLock()
	defer fake.nameToGuidMutex.RUnlock()
	fake.getLayerMountPathMutex.RLock()
	defer fake.getLayerMountPathMutex.RUnlock()
	fake.createContainerMutex.RLock()
	defer fake.createContainerMutex.RUnlock()
	fake.openContainerMutex.RLock()
	defer fake.openContainerMutex.RUnlock()
	fake.isPendingMutex.RLock()
	defer fake.isPendingMutex.RUnlock()
	fake.createSandboxLayerMutex.RLock()
	defer fake.createSandboxLayerMutex.RUnlock()
	fake.activateLayerMutex.RLock()
	defer fake.activateLayerMutex.RUnlock()
	fake.prepareLayerMutex.RLock()
	defer fake.prepareLayerMutex.RUnlock()
	fake.unprepareLayerMutex.RLock()
	defer fake.unprepareLayerMutex.RUnlock()
	fake.deactivateLayerMutex.RLock()
	defer fake.deactivateLayerMutex.RUnlock()
	fake.destroyLayerMutex.RLock()
	defer fake.destroyLayerMutex.RUnlock()
	fake.getContainerPropertiesMutex.RLock()
	defer fake.getContainerPropertiesMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeClient) recordInvocation(key string, args []interface{}) {
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

var _ hcsclient.Client = new(FakeClient)
