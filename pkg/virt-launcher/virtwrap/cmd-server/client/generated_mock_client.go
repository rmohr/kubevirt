// Automatically generated by MockGen. DO NOT EDIT!
// Source: client.go

package cmdclient

import (
	gomock "github.com/golang/mock/gomock"
	v1 "k8s.io/api/core/v1"

	v10 "kubevirt.io/kubevirt/pkg/api/v1"
	api "kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
)

// Mock of LauncherClient interface
type MockLauncherClient struct {
	ctrl     *gomock.Controller
	recorder *_MockLauncherClientRecorder
}

// Recorder for MockLauncherClient (not exported)
type _MockLauncherClientRecorder struct {
	mock *MockLauncherClient
}

func NewMockLauncherClient(ctrl *gomock.Controller) *MockLauncherClient {
	mock := &MockLauncherClient{ctrl: ctrl}
	mock.recorder = &_MockLauncherClientRecorder{mock}
	return mock
}

func (_m *MockLauncherClient) EXPECT() *_MockLauncherClientRecorder {
	return _m.recorder
}

func (_m *MockLauncherClient) StartVirtualMachine(vm *v10.VirtualMachine, secrets map[string]*v1.Secret) error {
	ret := _m.ctrl.Call(_m, "StartVirtualMachine", vm, secrets)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockLauncherClientRecorder) StartVirtualMachine(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "StartVirtualMachine", arg0, arg1)
}

func (_m *MockLauncherClient) ShutdownVirtualMachine(vm *v10.VirtualMachine) error {
	ret := _m.ctrl.Call(_m, "ShutdownVirtualMachine", vm)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockLauncherClientRecorder) ShutdownVirtualMachine(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "ShutdownVirtualMachine", arg0)
}

func (_m *MockLauncherClient) KillVirtualMachine(vm *v10.VirtualMachine) error {
	ret := _m.ctrl.Call(_m, "KillVirtualMachine", vm)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockLauncherClientRecorder) KillVirtualMachine(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "KillVirtualMachine", arg0)
}

func (_m *MockLauncherClient) SyncSecret(vm *v10.VirtualMachine, usageType string, usageID string, secretValue string) error {
	ret := _m.ctrl.Call(_m, "SyncSecret", vm, usageType, usageID, secretValue)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockLauncherClientRecorder) SyncSecret(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "SyncSecret", arg0, arg1, arg2, arg3)
}

func (_m *MockLauncherClient) ListDomains() ([]*api.Domain, error) {
	ret := _m.ctrl.Call(_m, "ListDomains")
	ret0, _ := ret[0].([]*api.Domain)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockLauncherClientRecorder) ListDomains() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "ListDomains")
}

func (_m *MockLauncherClient) Ping() error {
	ret := _m.ctrl.Call(_m, "Ping")
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockLauncherClientRecorder) Ping() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Ping")
}

func (_m *MockLauncherClient) Close() {
	_m.ctrl.Call(_m, "Close")
}

func (_mr *_MockLauncherClientRecorder) Close() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Close")
}
