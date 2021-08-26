package v1alpha2

// +k8s:openapi-gen=true
type CosignConfig struct {
	// +listType
	Annotations []CosignAnnotation `json:"annotations"`
}

// +k8s:openapi-gen=true
type CosignAnnotation struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
