package v1alpha2

type BuilderResource interface {
	GetName() string
	BuildBuilderSpec() BuildBuilderSpec
	Ready() bool
	BuildpackMetadata() BuildpackMetadataList
	RunImage() string
}
