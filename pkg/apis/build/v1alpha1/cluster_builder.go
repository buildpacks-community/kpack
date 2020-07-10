package v1alpha1

import v1 "k8s.io/api/core/v1"

func (c *ClusterBuilder) Image() string {
	return c.Spec.Image
}

func (c *ClusterBuilder) ImagePullSecrets() []v1.LocalObjectReference {
	return nil
}
