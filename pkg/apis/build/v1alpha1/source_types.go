package v1alpha1

type Source struct {
	Git Git `json:"git"`
}

type Git struct {
	URL      string `json:"url"`
	Revision string `json:"revision"`
}

type ResolvedSource struct {
	Git ResolvedGitSource `json:"git"`
}

const (
	Unknown GitSourceKind = "Unknown"
	Branch  GitSourceKind = "Branch"
	Tag     GitSourceKind = "Tag"
	Commit  GitSourceKind = "Commit"
)

type GitSourceKind string

type ResolvedGitSource struct {
	URL      string        `json:"url"`
	Revision string        `json:"commit"`
	Type     GitSourceKind `json:"type"`
}

func (gs *ResolvedGitSource) IsUnknown() bool {
	return gs.Type == Unknown
}

func (gs *ResolvedGitSource) IsPollable() bool {
	return gs.Type != Commit && gs.Type != Unknown
}
