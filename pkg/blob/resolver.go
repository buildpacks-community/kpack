package blob

import (
	"context"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

type Resolver struct {
}

func (*Resolver) Resolve(ctx context.Context, sourceResolver *buildapi.SourceResolver) (buildapi.ResolvedSourceConfig, error) {
	return buildapi.ResolvedSourceConfig{
		Blob: &buildapi.ResolvedBlobSource{
			URL:     sourceResolver.Spec.Source.Blob.URL,
			SubPath: sourceResolver.Spec.Source.SubPath,
		},
	}, nil
}

func (*Resolver) CanResolve(sourceResolver *buildapi.SourceResolver) bool {
	return sourceResolver.IsBlob()
}
