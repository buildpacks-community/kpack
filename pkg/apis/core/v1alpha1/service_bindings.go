package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

type ServiceBinding struct {
	Name      string
	SecretRef *corev1.LocalObjectReference
}

func (s *ServiceBinding) ServiceName() string {
	return s.Name
}

type CnbServiceBinding struct {
	Name        string
	SecretRef   *corev1.LocalObjectReference
	MetadataRef *corev1.LocalObjectReference
}

func (v *CnbServiceBinding) ServiceName() string {
	return v.Name
}
