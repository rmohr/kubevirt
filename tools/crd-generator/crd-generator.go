/*
 * This file is part of the KubeVirt project
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
 * Copyright 2018 Red Hat, Inc.
 *
 */

package main

import (
	"flag"
	"fmt"

	crdutils "github.com/ant31/crd-validation/pkg"

	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/kubevirt/pkg/api/v1"
)

func generateBlankCrd() *extensionsv1.CustomResourceDefinition {
	return &extensionsv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"kubevirt.io": "",
			},
		},
	}
}

func generateVirtualMachineCrd() {
	crd := generateBlankCrd()

	crd.ObjectMeta.Name = "virtualmachines." + v1.VirtualMachineGroupVersionKind.Group
	crd.Spec = extensionsv1.CustomResourceDefinitionSpec{
		Group:   v1.VirtualMachineGroupVersionKind.Group,
		Version: v1.VirtualMachineGroupVersionKind.Version,
		Scope:   "Namespaced",

		Names: extensionsv1.CustomResourceDefinitionNames{
			Plural:     "virtualmachines",
			Singular:   "virtualmachine",
			Kind:       v1.VirtualMachineGroupVersionKind.Kind,
			ShortNames: []string{"vm", "vms"},
		},
		Validation: crdutils.GetCustomResourceValidation("kubevirt.io/kubevirt/pkg/api/v1.VirtualMachine", v1.GetOpenAPIDefinitions),
	}

	crdutils.MarshallCrd(crd, "yaml")
}

func generatePresetCrd() {
	crd := generateBlankCrd()

	crd.ObjectMeta.Name = "virtualmachineinstancepresets." + v1.VirtualMachineInstancePresetGroupVersionKind.Group
	crd.Spec = extensionsv1.CustomResourceDefinitionSpec{
		Group:   v1.VirtualMachineInstancePresetGroupVersionKind.Group,
		Version: v1.VirtualMachineInstancePresetGroupVersionKind.Version,
		Scope:   "Namespaced",

		Names: extensionsv1.CustomResourceDefinitionNames{
			Plural:     "virtualmachineinstancepresets",
			Singular:   "virtualmachineinstancepreset",
			Kind:       v1.VirtualMachineInstancePresetGroupVersionKind.Kind,
			ShortNames: []string{"vmipreset", "vmipresets"},
		},
		Validation: crdutils.GetCustomResourceValidation("kubevirt.io/kubevirt/pkg/api/v1.VirtualMachineInstancePreset", v1.GetOpenAPIDefinitions),
	}

	crdutils.MarshallCrd(crd, "yaml")
}

func generateReplicaSetCrd() {
	crd := generateBlankCrd()

	crd.ObjectMeta.Name = "virtualmachineinstancereplicasets." + v1.VirtualMachineInstanceReplicaSetGroupVersionKind.Group
	crd.Spec = extensionsv1.CustomResourceDefinitionSpec{
		Group:   v1.VirtualMachineInstanceReplicaSetGroupVersionKind.Group,
		Version: v1.VirtualMachineInstanceReplicaSetGroupVersionKind.Version,
		Scope:   "Namespaced",

		Names: extensionsv1.CustomResourceDefinitionNames{
			Plural:     "virtualmachineinstancereplicasets",
			Singular:   "virtualmachineinstancereplicaset",
			Kind:       v1.VirtualMachineInstanceReplicaSetGroupVersionKind.Kind,
			ShortNames: []string{"vmirs", "vmirss"},
		},
		Validation: crdutils.GetCustomResourceValidation("kubevirt.io/kubevirt/pkg/api/v1.VirtualMachineInstanceReplicaSet", v1.GetOpenAPIDefinitions),
	}

	crdutils.MarshallCrd(crd, "yaml")
}

func generateVirtualMachineInstanceCrd() {
	crd := generateBlankCrd()

	crd.ObjectMeta.Name = "virtualmachineinstances." + v1.VirtualMachineInstanceGroupVersionKind.Group
	crd.Spec = extensionsv1.CustomResourceDefinitionSpec{
		Group:   v1.VirtualMachineInstanceGroupVersionKind.Group,
		Version: v1.VirtualMachineInstanceGroupVersionKind.Version,
		Scope:   "Namespaced",

		Names: extensionsv1.CustomResourceDefinitionNames{
			Plural:     "virtualmachineinstances",
			Singular:   "virtualmachineinstance",
			Kind:       v1.VirtualMachineInstanceGroupVersionKind.Kind,
			ShortNames: []string{"vmi", "vmis"},
		},
		Validation: crdutils.GetCustomResourceValidation("kubevirt.io/kubevirt/pkg/api/v1.VirtualMachineInstance", v1.GetOpenAPIDefinitions),
	}

	crdutils.MarshallCrd(crd, "yaml")
}

func main() {
	crdType := flag.String("crd-type", "", "Type of crd to generate. vmi | vmipreset | vmirs | vm")
	flag.Parse()

	switch *crdType {
	case "vmi":
		generateVirtualMachineInstanceCrd()
	case "vmipreset":
		generatePresetCrd()
	case "vmirs":
		generateReplicaSetCrd()
	case "vm":
		generateVirtualMachineCrd()
	default:
		panic(fmt.Errorf("unknown crd type %s", *crdType))
	}
}
