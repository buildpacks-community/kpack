/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type ProvisionedService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProvisionedServiceSpec   `json:"spec,omitempty"`
	Status ProvisionedServiceStatus `json:"status,omitempty"`
}

type ProvisionedServiceSpec struct {
	Binding corev1.LocalObjectReference `json:"binding,omitempty"`
}

type ProvisionedServiceStatus struct {
	duckv1.Status `json:",inline"`
	Binding       corev1.LocalObjectReference `json:"binding,omitempty"`
}

func (p *ProvisionedService) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("ProvisionedService")
}
