package s3

import "github.com/pivotal/kpack/pkg/apis/build/v1alpha1"

type Resolver struct {
}

func (*Resolver) Resolve(sourceResolver *v1alpha1.SourceResolver) (v1alpha1.ResolvedSourceConfig, error) {
	return v1alpha1.ResolvedSourceConfig{
		S3: &v1alpha1.ResolvedS3Source{
			URL:       sourceResolver.Spec.Source.S3.URL,
			AccessKey: sourceResolver.Spec.Source.S3.AccessKey,
			SecretKey: sourceResolver.Spec.Source.S3.SecretKey,
			Bucket:    sourceResolver.Spec.Source.S3.Bucket,
			File:      sourceResolver.Spec.Source.S3.File,
			SubPath:   sourceResolver.Spec.Source.SubPath,
		},
	}, nil
}

func (*Resolver) CanResolve(sourceResolver *v1alpha1.SourceResolver) bool {
	return sourceResolver.IsS3()
}
