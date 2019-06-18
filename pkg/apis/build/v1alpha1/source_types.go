package v1alpha1

type Source struct {
	Git Git `json:"git"`
}

type Git struct {
	URL      string `json:"url"`
	Revision string `json:"revision"`
}
