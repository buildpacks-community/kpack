package v1alpha1

import (
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

func (c *ClusterBuilder) Image() string {
	return c.Spec.Image
}

func (c *ClusterBuilder) BuildBuilderSpec() BuildBuilderSpec {
	return BuildBuilderSpec{
		Image: c.Status.LatestImage,
	}
}

func (c *ClusterBuilder) BuildpackMetadata() BuildpackMetadataList {
	return c.Status.BuilderMetadata
}

func (c *ClusterBuilder) Ready() bool {
	return c.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue() &&
		(c.Generation == c.Status.ObservedGeneration)
}

func (c *ClusterBuilder) ImagePullSecrets() []string {
	return nil
}
