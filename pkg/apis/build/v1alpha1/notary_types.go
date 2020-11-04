package v1alpha1

// +k8s:openapi-gen=true
type NotaryConfig struct {
	V1 *NotaryV1Config `json:"v1,omitempty"`
}

// +k8s:openapi-gen=true
type NotaryV1Config struct {
	URL             string           `json:"url"`
	SecretRef       NotarySecretRef  `json:"secretRef"`
	ConfigMapKeyRef *ConfigMapKeyRef `json:"configMapKeyRef,omitempty"`
}

// +k8s:openapi-gen=true
type NotarySecretRef struct {
	Name string `json:"name"`
}

// +k8s:openapi-gen=true
type ConfigMapKeyRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}
