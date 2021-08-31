package v1alpha1

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type NotaryConfig struct {
	V1 *NotaryV1Config `json:"v1,omitempty"`
}

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type NotaryV1Config struct {
	URL       string          `json:"url"`
	SecretRef NotarySecretRef `json:"secretRef"`
}

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type NotarySecretRef struct {
	Name string `json:"name"`
}
