package v1alpha1

type BuildpackMetadataList []BuildpackMetadata

// +k8s:openapi-gen=true
type BuildpackMetadata struct {
	Key     string `json:"key"`
	Version string `json:"version"`
}

func (l BuildpackMetadataList) Include(q BuildpackMetadata) bool {
	for _, bp := range l {
		if bp.Key == q.Key && bp.Version == q.Version {
			return true
		}
	}

	return false
}
