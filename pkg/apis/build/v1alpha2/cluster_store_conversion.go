package v1alpha2

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func (s *ClusterStore) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toClusterStore := to.(type) {
	case *v1alpha1.ClusterStore:
		toClusterStore.ObjectMeta = s.ObjectMeta

		s.Spec.convertTo(&toClusterStore.Spec)
		s.Status.convertTo(&toClusterStore.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", toClusterStore)
	}
	return nil
}

func (s *ClusterStore) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromClusterStore := from.(type) {
	case *v1alpha1.ClusterStore:
		s.ObjectMeta = fromClusterStore.ObjectMeta
		s.Spec.convertFrom(&fromClusterStore.Spec)
		s.Status.convertFrom(&fromClusterStore.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", fromClusterStore)
	}

	return nil
}

func (cs *ClusterStoreSpec) convertTo(to *v1alpha1.ClusterStoreSpec) {
	to.Sources = cs.Sources
}

func (cs *ClusterStoreSpec) convertFrom(from *v1alpha1.ClusterStoreSpec) {
	cs.Sources = from.Sources
}

func (ct *ClusterStoreStatus) convertTo(to *v1alpha1.ClusterStoreStatus) {
	to.Status = ct.Status
	to.Buildpacks = ct.Buildpacks
}

func (ct *ClusterStoreStatus) convertFrom(from *v1alpha1.ClusterStoreStatus) {
	ct.Status = from.Status
	ct.Buildpacks = from.Buildpacks
}
