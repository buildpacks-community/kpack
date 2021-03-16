package v1alpha1

type BuilderResource interface {
	GetName() string
	BuildBuilderSpec() BuildBuilderSpec
	Ready() bool
	BuildpackMetadata() BuildpackMetadataList
	RunImage() string
	GetKind() string
}
