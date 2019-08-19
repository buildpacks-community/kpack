package registry

import "github.com/pivotal/kpack/pkg/apis/build/v1alpha1"

type Resolver struct {
}

func (*Resolver) Resolve(sourceResolver *v1alpha1.SourceResolver) (v1alpha1.ResolvedSourceConfig, error) {
	return v1alpha1.ResolvedSourceConfig{
		Registry: &v1alpha1.ResolvedRegistrySource{
			Image:            sourceResolver.Spec.Source.Registry.Image,
			ImagePullSecrets: sourceResolver.Spec.Source.Registry.ImagePullSecrets,
		},
	}, nil
}

func (*Resolver) CanResolve(sourceResolver *v1alpha1.SourceResolver) bool {
	return sourceResolver.IsRegistry()
}
