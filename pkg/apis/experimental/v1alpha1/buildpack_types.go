package v1alpha1

type Group struct {
	Group []Buildpack `json:"group"`
}

type Buildpack struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Optional bool   `json:"optional"`
}
