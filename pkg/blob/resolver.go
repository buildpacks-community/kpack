package blob

import (
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

type Resolver struct {
}

func (*Resolver) Resolve(sourceResolver *v1alpha2.SourceResolver) (v1alpha2.ResolvedSourceConfig, error) {
	return v1alpha2.ResolvedSourceConfig{
		Blob: &v1alpha2.ResolvedBlobSource{
			URL:     sourceResolver.Spec.Source.Blob.URL,
			SubPath: sourceResolver.Spec.Source.SubPath,
		},
	}, nil
}

func (*Resolver) CanResolve(sourceResolver *v1alpha2.SourceResolver) bool {
	return sourceResolver.IsBlob()
}
