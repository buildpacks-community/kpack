package v1alpha2

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func (i *Image) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toI := to.(type) {
	case *v1alpha1.Image:
		toI.TypeMeta = i.TypeMeta
		toI.ObjectMeta = i.ObjectMeta
		toI.Spec.Tag = i.Spec.Tag
		toI.Spec.Builder = i.Spec.Builder
		toI.Spec.ServiceAccount = i.Spec.ServiceAccount
		toI.Spec.Source = i.Spec.Source
		toI.Spec.CacheSize = i.Spec.CacheSize
		toI.Spec.FailedBuildHistoryLimit = i.Spec.FailedBuildHistoryLimit
		toI.Spec.SuccessBuildHistoryLimit = i.Spec.SuccessBuildHistoryLimit
		toI.Spec.ImageTaggingStrategy = v1alpha1.ImageTaggingStrategy(i.Spec.ImageTaggingStrategy)
		toI.Spec.Build = convertBuildTo(i.Spec.Build)
	default:
		return fmt.Errorf("unsupported type %T", toI)
	}
	return nil
}

func convertBuildTo(bld *ImageBuild) *v1alpha1.ImageBuild {
	if bld == nil {
		return nil
	}

	v1Bld := v1alpha1.ImageBuild{
		Env:       bld.Env,
		Resources: bld.Resources,
	}

	for _, b := range bld.Services {
		v1Bld.Bindings = append(v1Bld.Bindings, v1alpha1.Binding{
			Name: b.Name,
			SecretRef: &v1.LocalObjectReference{
				Name: b.Name,
			},
		})
	}

	return &v1Bld
}

func (i *Image) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromI := from.(type) {
	case *v1alpha1.Image:
		i.TypeMeta = fromI.TypeMeta
		i.ObjectMeta = fromI.ObjectMeta
		i.Spec.Tag = fromI.Spec.Tag
		i.Spec.Builder = fromI.Spec.Builder
		i.Spec.ServiceAccount = fromI.Spec.ServiceAccount
		i.Spec.Source = fromI.Spec.Source
		i.Spec.CacheSize = fromI.Spec.CacheSize
		i.Spec.FailedBuildHistoryLimit = fromI.Spec.FailedBuildHistoryLimit
		i.Spec.SuccessBuildHistoryLimit = fromI.Spec.SuccessBuildHistoryLimit
		i.Spec.ImageTaggingStrategy = ImageTaggingStrategy(fromI.Spec.ImageTaggingStrategy)
		i.Spec.Build = convertBuildFrom(fromI.Spec.Build)

		bindings, err := json.Marshal(fromI.Spec.Build.Bindings)
		if err != nil {
			return err
		}

		if i.ObjectMeta.Annotations == nil {
			i.ObjectMeta.Annotations = map[string]string{}
		}
		i.ObjectMeta.Annotations[V1Alpha1BindingsAnnotation] = string(bindings)
	default:
		return fmt.Errorf("unsupported type %T", fromI)
	}
	return nil
}

func convertBuildFrom(v1Bld *v1alpha1.ImageBuild) *ImageBuild {
	if v1Bld == nil {
		return nil
	}

	return &ImageBuild{
		Env:       v1Bld.Env,
		Resources: v1Bld.Resources,
	}
}
