package registry

import (
	"context"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

type Resolver struct {
}

func (*Resolver) Resolve(ctx context.Context, sourceResolver *buildapi.SourceResolver) (corev1alpha1.ResolvedSourceConfig, error) {
	return corev1alpha1.ResolvedSourceConfig{
		Registry: &corev1alpha1.ResolvedRegistrySource{
			Image:            sourceResolver.Spec.Source.Registry.Image,
			ImagePullSecrets: sourceResolver.Spec.Source.Registry.ImagePullSecrets,
			SubPath:          sourceResolver.Spec.Source.SubPath,
		},
	}, nil
}

func (*Resolver) CanResolve(sourceResolver *buildapi.SourceResolver) bool {
	return sourceResolver.IsRegistry()
}
