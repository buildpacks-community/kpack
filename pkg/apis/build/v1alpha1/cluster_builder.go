package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
)

func (in *ClusterBuilder) ServiceAccount() string {
	return ""
}

func (in *ClusterBuilder) Namespace() string {
	return ""
}

func (in *ClusterBuilder) Identifier() string {
	return in.Spec.Image
}

func (in *ClusterBuilder) HasSecret() bool {
	return false
}

func (in *ClusterBuilder) SecretName() string {
	return ""
}

func (in *ClusterBuilder) ImageRef() BuilderImage {
	return BuilderImage{
		Image:            in.Status.LatestImage,
		ImagePullSecrets: nil,
	}
}

func (in *ClusterBuilder) BuildpackMetadata() BuildpackMetadataList {
	return in.Status.BuilderMetadata
}

func (in *ClusterBuilder) Ready() bool {
	return in.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue() &&
		(in.Generation == in.Status.ObservedGeneration)
}
