package cnb

const (
	buildpackageMetadataLabel = "io.buildpacks.buildpackage.metadata"
)

type BuildpackageMetadata struct {
	Id       string `json:"id"`
	Version  string `json:"version"`
	Homepage string `json:"homepage"`
}
