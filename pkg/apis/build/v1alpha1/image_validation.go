package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"
)

const (
	defaultServiceAccount = "default"
)

var (
	defaultFailedBuildHistoryLimit     int64 = 10
	defaultSuccessfulBuildHistoryLimit int64 = 10
)

func (s *Image) SetDefaults(ctx context.Context) {
	if s.Spec.ServiceAccount == "" {
		s.Spec.ServiceAccount = defaultServiceAccount
	}

	if s.Spec.ImageTaggingStrategy == "" {
		s.Spec.ImageTaggingStrategy = BuildNumber
	}

	if s.Spec.FailedBuildHistoryLimit == nil {
		s.Spec.FailedBuildHistoryLimit = &defaultFailedBuildHistoryLimit
	}

	if s.Spec.SuccessBuildHistoryLimit == nil {
		s.Spec.SuccessBuildHistoryLimit = &defaultSuccessfulBuildHistoryLimit
	}
}

func (s *Image) Validate(ctx context.Context) *apis.FieldError {
	return s.Spec.Validate(ctx).ViaField("spec")
}

func (is *ImageSpec) Validate(ctx context.Context) *apis.FieldError {
	return is.validateTag(ctx).
		Also(is.Builder.Validate(ctx).ViaField("builder")).
		Also(is.Source.Validate(ctx).ViaField("source"))
}

func (im *ImageSpec) validateTag(ctx context.Context) *apis.FieldError {
	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Image)
		return validateImmutableField(original.Spec.Tag, im.Tag, "tag")
	}

	return validateTag(im.Tag)
}

func (im *ImageBuilder) Validate(ctx context.Context) *apis.FieldError {
	if im.Name == "" {
		return apis.ErrMissingField("name")
	}

	switch im.Kind {
	case ClusterBuilderKind,
		"CustomBuilder", // TODO : use the const var when the experimental pkg migrates into the build pkg 
		BuilderKind:
		return nil
	default:
		return apis.ErrInvalidValue(im.Kind, "kind")
	}
}

func (s *SourceConfig) Validate(ctx context.Context) *apis.FieldError {
	sources := make([]string, 0, 3)
	if s.Git != nil {
		sources = append(sources, "git")
	}
	if s.Blob != nil {
		sources = append(sources, "blob")
	}
	if s.Registry != nil {
		sources = append(sources, "registry")
	}

	if len(sources) == 0 {
		return apis.ErrMissingOneOf("git", "blob", "registry")
	}

	if len(sources) != 1 {
		return apis.ErrMultipleOneOf(sources...)
	}

	return (s.Git.Validate(ctx).ViaField("git")).
		Also(s.Blob.Validate(ctx).ViaField("blob")).
		Also(s.Registry.Validate(ctx).ViaField("registry"))
}

func (g *Git) Validate(ctx context.Context) *apis.FieldError {
	if g == nil {
		return nil
	}

	return validateFieldNotEmpty(g.URL, "url").
		Also(validateFieldNotEmpty(g.Revision, "revision"))
}

func (b *Blob) Validate(ctx context.Context) *apis.FieldError {
	if b == nil {
		return nil
	}

	return validateFieldNotEmpty(b.URL, "url")
}

func (r *Registry) Validate(ctx context.Context) *apis.FieldError {
	if r == nil {
		return nil
	}

	return validateImage(r.Image)
}
