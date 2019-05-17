package v1alpha1

type CNBBuildpackMetadataList []CNBBuildpackMetadata

type CNBBuildpackMetadata struct {
	ID      string `json:"key"`
	Version string `json:"version"`
}

func (l CNBBuildpackMetadataList) Include(q CNBBuildpackMetadata) bool {
	for _, bp := range l {
		if bp.ID == q.ID && bp.Version == q.Version {
			return true
		}
	}

	return false
}
