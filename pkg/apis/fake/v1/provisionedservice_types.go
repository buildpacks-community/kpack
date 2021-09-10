/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:deepcopy-gen=true
type FakeProvisionedService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProvisionedServiceSpec   `json:"spec,omitempty"`
	Status ProvisionedServiceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen=true
type ProvisionedServiceSpec struct {
	//random spec
	DatabaseSize string `json:"databaseSize,omitempty"`
}

// +k8s:deepcopy-gen=true
type ProvisionedServiceStatus struct {
	Binding corev1.LocalObjectReference `json:"binding,omitempty"`
}
