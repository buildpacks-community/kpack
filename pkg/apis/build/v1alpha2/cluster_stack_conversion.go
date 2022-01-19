package v1alpha2

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func (s *ClusterStack) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toClusterStack := to.(type) {
	case *v1alpha1.ClusterStack:
		toClusterStack.ObjectMeta = s.ObjectMeta

		s.Spec.convertTo(&toClusterStack.Spec)
		s.Status.convertTo(&toClusterStack.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", toClusterStack)
	}
	return nil
}

func (s *ClusterStack) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromClusterStack := from.(type) {
	case *v1alpha1.ClusterStack:
		s.ObjectMeta = fromClusterStack.ObjectMeta
		s.Spec.convertFrom(&fromClusterStack.Spec)
		s.Status.convertFrom(&fromClusterStack.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", fromClusterStack)
	}

	return nil
}

func (cs *ClusterStackSpec) convertTo(to *v1alpha1.ClusterStackSpec) {
	to.Id = cs.Id
	to.BuildImage.Image = cs.BuildImage.Image
	to.RunImage.Image = cs.RunImage.Image
}

func (cs *ClusterStackSpec) convertFrom(from *v1alpha1.ClusterStackSpec) {
	cs.Id = from.Id
	cs.BuildImage.Image = from.BuildImage.Image
	cs.RunImage.Image = from.RunImage.Image
}

func (ct *ClusterStackStatus) convertTo(to *v1alpha1.ClusterStackStatus) {
	to.Status = ct.Status
	to.Id = ct.Id
	to.BuildImage = v1alpha1.ClusterStackStatusImage{
		LatestImage: ct.BuildImage.LatestImage,
		Image:       ct.BuildImage.Image,
	}
	to.RunImage = v1alpha1.ClusterStackStatusImage{
		LatestImage: ct.RunImage.LatestImage,
		Image:       ct.RunImage.Image,
	}
	to.Mixins = ct.Mixins
	to.UserID = ct.UserID
	to.GroupID = ct.GroupID
}

func (ct *ClusterStackStatus) convertFrom(from *v1alpha1.ClusterStackStatus) {
	ct.Status = from.Status
	ct.Id = from.Id
	ct.BuildImage = ClusterStackStatusImage{
		LatestImage: from.BuildImage.LatestImage,
		Image:       from.BuildImage.Image,
	}
	ct.RunImage = ClusterStackStatusImage{
		LatestImage: from.RunImage.LatestImage,
		Image:       from.RunImage.Image,
	}
	ct.Mixins = from.Mixins
	ct.UserID = from.UserID
	ct.GroupID = from.GroupID
}
