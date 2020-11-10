package registry

import (
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

type Resolver struct {
}

func (*Resolver) Resolve(sourceResolver *v1alpha2.SourceResolver) (v1alpha2.ResolvedSourceConfig, error) {
	return v1alpha2.ResolvedSourceConfig{
		Registry: &v1alpha2.ResolvedRegistrySource{
			Image:            sourceResolver.Spec.Source.Registry.Image,
			ImagePullSecrets: sourceResolver.Spec.Source.Registry.ImagePullSecrets,
			SubPath:          sourceResolver.Spec.Source.SubPath,
		},
	}, nil
}

func (*Resolver) CanResolve(sourceResolver *v1alpha2.SourceResolver) bool {
	return sourceResolver.IsRegistry()
}
