/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type FakeProvisionedService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProvisionedServiceSpec   `json:"spec,omitempty"`
	Status ProvisionedServiceStatus `json:"status,omitempty"`
}

func (ps *FakeProvisionedService) DeepCopyObject() runtime.Object {
	return &FakeProvisionedService{
		TypeMeta:   ps.TypeMeta,
		ObjectMeta: ps.ObjectMeta,
		Spec:       ps.Spec,
		Status:     ps.Status,
	}
}

type ProvisionedServiceSpec struct {
	//random spec
	DatabaseSize string `json:"databaseSize,omitempty"`
}

type ProvisionedServiceStatus struct {
	Binding corev1.LocalObjectReference `json:"binding,omitempty"`
}
