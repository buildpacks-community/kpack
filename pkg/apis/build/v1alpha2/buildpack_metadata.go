package v1alpha2

type BuildpackMetadataList []BuildpackMetadata

// +k8s:openapi-gen=true
type BuildpackMetadata struct {
	Id       string `json:"id"`
	Version  string `json:"version"`
	Homepage string `json:"homepage,omitempty"`
}

func (l BuildpackMetadataList) Include(q BuildpackMetadata) bool {
	for _, bp := range l {
		if bp.Id == q.Id && bp.Version == q.Version {
			return true
		}
	}

	return false
}
