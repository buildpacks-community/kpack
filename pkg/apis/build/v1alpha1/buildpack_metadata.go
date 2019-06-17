package v1alpha1

type BuildpackMetadataList []BuildpackMetadata

type BuildpackMetadata struct {
	ID      string `json:"key"`
	Version string `json:"version"`
}

func (l BuildpackMetadataList) Include(q BuildpackMetadata) bool {
	for _, bp := range l {
		if bp.ID == q.ID && bp.Version == q.Version {
			return true
		}
	}

	return false
}
