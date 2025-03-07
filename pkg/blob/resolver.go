package blob

import (
	"context"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

type Resolver struct {
}

func (*Resolver) Resolve(ctx context.Context, sourceResolver *buildapi.SourceResolver) (corev1alpha1.ResolvedSourceConfig, error) {
	return corev1alpha1.ResolvedSourceConfig{
		Blob: &corev1alpha1.ResolvedBlobSource{
			URL:     sourceResolver.Spec.Source.Blob.URL,
			Auth:    sourceResolver.Spec.Source.Blob.Auth,
			SubPath: sourceResolver.Spec.Source.SubPath,
		},
	}, nil
}

func (*Resolver) CanResolve(sourceResolver *buildapi.SourceResolver) bool {
	return sourceResolver.IsBlob()
}
