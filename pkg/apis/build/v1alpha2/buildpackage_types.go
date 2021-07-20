package v1alpha2

// +k8s:openapi-gen=true
type BuildpackageInfo struct {
	Id       string `json:"id,omitempty"`
	Version  string `json:"version,omitempty"`
	Homepage string `json:"homepage,omitempty"`
}
