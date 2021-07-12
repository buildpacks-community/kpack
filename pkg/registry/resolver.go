package registry

import (
	"context"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

type Resolver struct {
}

func (*Resolver) Resolve(ctx context.Context, sourceResolver *buildapi.SourceResolver) (buildapi.ResolvedSourceConfig, error) {
	return buildapi.ResolvedSourceConfig{
		Registry: &buildapi.ResolvedRegistrySource{
			Image:            sourceResolver.Spec.Source.Registry.Image,
			ImagePullSecrets: sourceResolver.Spec.Source.Registry.ImagePullSecrets,
			SubPath:          sourceResolver.Spec.Source.SubPath,
		},
	}, nil
}

func (*Resolver) CanResolve(sourceResolver *buildapi.SourceResolver) bool {
	return sourceResolver.IsRegistry()
}
