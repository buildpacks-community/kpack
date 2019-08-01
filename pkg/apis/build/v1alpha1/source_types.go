package v1alpha1

type Source struct {
	Git  *Git  `json:"git,omitempty"`
	Blob *Blob `json:"blob,omitempty"`
}

func (s Source) IsGit() bool {
	return s.Git != nil
}

func (s Source) IsBlob() bool {
	return s.Blob != nil
}

type Git struct {
	URL      string `json:"url"`
	Revision string `json:"revision"`
}

type Blob struct {
	URL string `json:"url"`
}

type ResolvedSource struct {
	Git  *ResolvedGitSource  `json:"git,omitempty"`
	Blob *ResolvedBlobSource `json:"blob,omitempty"`
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

type ResolvedBlobSource struct {
	URL string `json:"url"`
}
