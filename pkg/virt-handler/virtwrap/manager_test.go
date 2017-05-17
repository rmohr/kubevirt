/*
 * This file is part of the kubevirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2017 Red Hat, Inc.
 *
 */

package virtwrap

import (
	"encoding/xml"

	"github.com/golang/mock/gomock"
	"github.com/jeevatkm/go-model"
	"github.com/libvirt/libvirt-go"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	kubev1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/record"

	"k8s.io/client-go/pkg/types"

	"kubevirt.io/kubevirt/pkg/api/v1"
	"kubevirt.io/kubevirt/pkg/logging"
	"kubevirt.io/kubevirt/pkg/virt-handler/virtwrap/api"
)

var _ = Describe("Manager", func() {
	var mockConn *MockConnection
	var mockDomain *MockVirDomain
	var ctrl *gomock.Controller
	var recorder *record.FakeRecorder

	logging.DefaultLogger().SetIOWriter(GinkgoWriter)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockConn = NewMockConnection(ctrl)
		mockDomain = NewMockVirDomain(ctrl)
		recorder = record.NewFakeRecorder(10)

		// Make sure that we always free the domain after use
		mockDomain.EXPECT().Free()
	})

	Context("on successful VM sync", func() {
		It("should define and start a new VM", func() {
			vm := newVM("testvm")
			mockConn.EXPECT().LookupDomainByName("testvm").Return(nil, libvirt.Error{Code: libvirt.ERR_NO_DOMAIN})

			// we have to make sure that we use correct DomainSpec (from virtwrap)
			var domainSpec api.DomainSpec
			Expect(model.Copy(&domainSpec, vm.Spec.Domain)).To(BeEmpty())

			xml, err := xml.Marshal(domainSpec)
			Expect(err).To(BeNil())
			mockConn.EXPECT().DomainDefineXML(string(xml)).Return(mockDomain, nil)
			mockDomain.EXPECT().GetState().Return(libvirt.DOMAIN_SHUTDOWN, 1, nil)
			mockDomain.EXPECT().Create().Return(nil)
			manager, _ := NewLibvirtDomainManager(mockConn, recorder)
			err = manager.SyncVM(vm)
			Expect(err).To(BeNil())
			Expect(<-recorder.Events).To(ContainSubstring(v1.Created.String()))
			Expect(<-recorder.Events).To(ContainSubstring(v1.Started.String()))
			Expect(recorder.Events).To(BeEmpty())
		})
		It("should detect mismatching UIDs on VMs and remove the old domain", func() {
			vm := newVM("testvm")

			// Set mismatching UID
			vm.GetObjectMeta().SetUID("cba")

			oldMockDomain := NewMockVirDomain(ctrl)
			oldMockDomain.EXPECT().Free()
			// Expect calls which check if the UIDs match
			oldMockDomain.EXPECT().GetUUIDString().Return("abc", nil)
			// Expect the Destroy call for the outdated domain
			oldMockDomain.EXPECT().GetState().Return(libvirt.DOMAIN_RUNNING, 1, nil)
			oldMockDomain.EXPECT().Destroy()
			oldMockDomain.EXPECT().Undefine()

			mockConn.EXPECT().LookupDomainByName("testvm").Return(oldMockDomain, nil)

			// we have to make sure that we use correct DomainSpec (from virtwrap)
			var domainSpec api.DomainSpec
			Expect(model.Copy(&domainSpec, vm.Spec.Domain)).To(BeEmpty())

			xml, err := xml.Marshal(domainSpec)
			Expect(err).To(BeNil())
			mockConn.EXPECT().DomainDefineXML(string(xml)).Return(mockDomain, nil)
			mockDomain.EXPECT().GetState().Return(libvirt.DOMAIN_SHUTDOWN, 1, nil)
			mockDomain.EXPECT().Create().Return(nil)
			manager, _ := NewLibvirtDomainManager(mockConn, recorder)
			err = manager.SyncVM(vm)
			Expect(err).To(BeNil())
			Expect(<-recorder.Events).To(ContainSubstring(v1.Stopped.String()))
			Expect(<-recorder.Events).To(ContainSubstring(v1.Deleted.String()))
			Expect(<-recorder.Events).To(ContainSubstring(v1.Created.String()))
			Expect(<-recorder.Events).To(ContainSubstring(v1.Started.String()))
			Expect(recorder.Events).To(BeEmpty())
		})
		It("should leave a defined and started VM alone", func() {
			vm := newVM("testvm")
			vm.GetObjectMeta().SetUID(types.UID("123"))
			mockConn.EXPECT().LookupDomainByName("testvm").Return(mockDomain, nil)
			mockDomain.EXPECT().GetUUIDString().Return(string(vm.GetObjectMeta().GetUID()), nil)
			mockDomain.EXPECT().GetState().Return(libvirt.DOMAIN_RUNNING, 1, nil)
			manager, _ := NewLibvirtDomainManager(mockConn, recorder)
			err := manager.SyncVM(vm)
			Expect(err).To(BeNil())
			Expect(recorder.Events).To(BeEmpty())
		})
		table.DescribeTable("should try to start a VM in state",
			func(state libvirt.DomainState) {
				vm := newVM("testvm")
				vm.GetObjectMeta().SetUID(types.UID("123"))
				mockConn.EXPECT().LookupDomainByName("testvm").Return(mockDomain, nil)
				mockDomain.EXPECT().GetUUIDString().Return(string(vm.GetObjectMeta().GetUID()), nil)
				mockDomain.EXPECT().GetState().Return(state, 1, nil)
				mockDomain.EXPECT().Create().Return(nil)
				manager, _ := NewLibvirtDomainManager(mockConn, recorder)
				err := manager.SyncVM(vm)
				Expect(err).To(BeNil())
				Expect(<-recorder.Events).To(ContainSubstring(v1.Started.String()))
				Expect(recorder.Events).To(BeEmpty())
			},
			table.Entry("crashed", libvirt.DOMAIN_CRASHED),
			table.Entry("shutdown", libvirt.DOMAIN_SHUTDOWN),
			table.Entry("shutoff", libvirt.DOMAIN_SHUTOFF),
			table.Entry("unknown", libvirt.DOMAIN_NOSTATE),
		)
		It("should resume a paused VM", func() {
			vm := newVM("testvm")
			vm.GetObjectMeta().SetUID(types.UID("123"))
			mockConn.EXPECT().LookupDomainByName("testvm").Return(mockDomain, nil)
			mockDomain.EXPECT().GetUUIDString().Return(string(vm.GetObjectMeta().GetUID()), nil)
			mockDomain.EXPECT().GetState().Return(libvirt.DOMAIN_PAUSED, 1, nil)
			mockDomain.EXPECT().Resume().Return(nil)
			manager, _ := NewLibvirtDomainManager(mockConn, recorder)
			err := manager.SyncVM(vm)
			Expect(err).To(BeNil())
			Expect(<-recorder.Events).To(ContainSubstring(v1.Resumed.String()))
			Expect(recorder.Events).To(BeEmpty())
		})
	})

	Context("on successful VM kill", func() {
		table.DescribeTable("should try to undefine a VM in state",
			func(state libvirt.DomainState) {
				mockConn.EXPECT().LookupDomainByName("testvm").Return(mockDomain, nil)
				mockDomain.EXPECT().GetState().Return(state, 1, nil)
				mockDomain.EXPECT().Undefine().Return(nil)
				manager, _ := NewLibvirtDomainManager(mockConn, recorder)
				err := manager.KillVM(newVM("testvm"))
				Expect(err).To(BeNil())
			},
			table.Entry("crashed", libvirt.DOMAIN_CRASHED),
			table.Entry("shutdown", libvirt.DOMAIN_SHUTDOWN),
			table.Entry("shutoff", libvirt.DOMAIN_SHUTOFF),
			table.Entry("unknown", libvirt.DOMAIN_NOSTATE),
		)
		table.DescribeTable("should try to destroy and undefine a VM in state",
			func(state libvirt.DomainState) {
				mockConn.EXPECT().LookupDomainByName("testvm").Return(mockDomain, nil)
				mockDomain.EXPECT().GetState().Return(state, 1, nil)
				mockDomain.EXPECT().Destroy().Return(nil)
				mockDomain.EXPECT().Undefine().Return(nil)
				manager, _ := NewLibvirtDomainManager(mockConn, recorder)
				err := manager.KillVM(newVM("testvm"))
				Expect(err).To(BeNil())
				Expect(<-recorder.Events).To(ContainSubstring(v1.Stopped.String()))
				Expect(<-recorder.Events).To(ContainSubstring(v1.Deleted.String()))
				Expect(recorder.Events).To(BeEmpty())
			},
			table.Entry("running", libvirt.DOMAIN_RUNNING),
			table.Entry("paused", libvirt.DOMAIN_PAUSED),
		)
	})

	// TODO: test error reporting on non successful VM syncs and kill attempts

	AfterEach(func() {
		ctrl.Finish()
	})
})

func newVM(name string) *v1.VM {
	return &v1.VM{
		ObjectMeta: kubev1.ObjectMeta{Name: name},
		Spec:       v1.VMSpec{Domain: v1.NewMinimalDomainSpec(name)},
	}
}
