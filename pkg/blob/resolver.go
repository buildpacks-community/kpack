package blob

import "github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"

type Resolver struct {
}

func (b *Resolver) Resolve(sourceResolver *v1alpha1.SourceResolver) (v1alpha1.ResolvedSource, error) {
	return &v1alpha1.ResolvedBlobSource{
		URL: sourceResolver.Spec.Source.Blob.URL,
	}, nil
}

func (g *Resolver) CanResolve(sourceResolver *v1alpha1.SourceResolver) bool {
	return sourceResolver.IsBlob()
}
