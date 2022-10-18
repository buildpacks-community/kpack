package v1alpha2

import (
	"context"
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

const (
	clusterStoreServiceAccountRefAnnotation = "kpack.io/clusterStoreServiceAccountRef"
)

func (s *ClusterStore) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toClusterStore := to.(type) {
	case *v1alpha1.ClusterStore:
		s.ObjectMeta.DeepCopyInto(&toClusterStore.ObjectMeta)

		if toClusterStore.Annotations == nil {
			toClusterStore.Annotations = map[string]string{}
		}

		s.Spec.convertTo(&toClusterStore.Spec)
		s.Status.convertTo(&toClusterStore.Status)

		if err := s.Spec.convertToAnnotations(toClusterStore.Annotations); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown version, got: %T", toClusterStore)
	}
	return nil
}

func (cs *ClusterStoreSpec) convertToAnnotations(toAnnotations map[string]string) error {
	if cs.ServiceAccountRef != nil {
		bytes, err := json.Marshal(cs.ServiceAccountRef)
		if err!= nil {
			return err
		}
		toAnnotations[clusterStoreServiceAccountRefAnnotation] = string(bytes)
	}
	return nil
}

func (s *ClusterStore) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromClusterStore := from.(type) {
	case *v1alpha1.ClusterStore:
		fromClusterStore.ObjectMeta.DeepCopyInto(&s.ObjectMeta)
		s.Spec.convertFrom(&fromClusterStore.Spec)
		s.Status.convertFrom(&fromClusterStore.Status)
		s.convertFromAnnotations(&fromClusterStore.Annotations)
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

func (s *ClusterStore) convertFromAnnotations(fromAnnotations *map[string]string) error {
	if serviceAccountRefJson, ok := (*fromAnnotations)[clusterStoreServiceAccountRefAnnotation]; ok {
		var serviceAccountRef *corev1.ObjectReference
		if err := json.Unmarshal([]byte(serviceAccountRefJson), &serviceAccountRef); err != nil {
			return err
		}
		s.Spec.ServiceAccountRef = serviceAccountRef
		delete(s.Annotations, clusterStoreServiceAccountRefAnnotation)
	}
	return nil
}
