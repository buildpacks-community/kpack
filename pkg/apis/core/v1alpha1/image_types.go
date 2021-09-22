package v1alpha1

type ImageTaggingStrategy string

const (
	None        ImageTaggingStrategy = "None"
	BuildNumber ImageTaggingStrategy = "BuildNumber"
)
