package v1alpha2

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

const V1Alpha1BindingsAnnotation = "kpack.io/v1alpha1Bindings"

func (b *Build) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toB := to.(type) {
	case *v1alpha1.Build:
		toB.TypeMeta = b.TypeMeta
		toB.ObjectMeta = b.ObjectMeta
		toB.Spec.Tags = b.Spec.Tags
		toB.Spec.Builder = b.Spec.Builder
		toB.Spec.ServiceAccount = b.Spec.ServiceAccount
		toB.Spec.Source = b.Spec.Source
		toB.Spec.CacheName = b.Spec.CacheName
		toB.Spec.LastBuild = convertLastBuildTo(b.Spec.LastBuild)
		toB.Spec.Env = b.Spec.Env
		toB.Spec.Resources = b.Spec.Resources

		for _, s := range b.Spec.Services {
			toB.Spec.Bindings = append(toB.Spec.Bindings, v1alpha1.Binding{
				Name: s.Name,
				SecretRef: &v1.LocalObjectReference{
					Name: s.Name,
				},
			})
		}
	default:
		return fmt.Errorf("unsupported type %T", toB)
	}
	return nil
}

func convertLastBuildTo(bld *LastBuild) *v1alpha1.LastBuild {
	if bld == nil {
		return nil
	}
	return &v1alpha1.LastBuild{
		Image:   bld.Image,
		StackId: bld.StackId,
	}
}

func (b *Build) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromB := from.(type) {
	case *v1alpha1.Build:
		b.TypeMeta = fromB.TypeMeta
		b.ObjectMeta = fromB.ObjectMeta
		b.Spec.Tags = fromB.Spec.Tags
		b.Spec.Builder = fromB.Spec.Builder
		b.Spec.ServiceAccount = fromB.Spec.ServiceAccount
		b.Spec.Source = fromB.Spec.Source
		b.Spec.CacheName = fromB.Spec.CacheName
		b.Spec.LastBuild = convertLastBuildFrom(fromB.Spec.LastBuild)
		b.Spec.Env = fromB.Spec.Env
		b.Spec.Resources = fromB.Spec.Resources

		bindings, err := json.Marshal(fromB.Spec.Bindings)
		if err != nil {
			return err
		}

		if b.ObjectMeta.Annotations == nil {
			b.ObjectMeta.Annotations = map[string]string{}
		}
		b.ObjectMeta.Annotations[V1Alpha1BindingsAnnotation] = string(bindings)
	default:
		return fmt.Errorf("unsupported type %T", fromB)
	}
	return nil
}

func convertLastBuildFrom(bld *v1alpha1.LastBuild) *LastBuild {
	if bld == nil {
		return nil
	}
	return &LastBuild{
		Image:   bld.Image,
		StackId: bld.StackId,
	}
}
