package build

import (
	_ "strconv"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

type ByCreationTimestamp []*buildapi.Build

func (o ByCreationTimestamp) Len() int      { return len(o) }
func (o ByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

func (o ByCreationTimestamp) Less(i, j int) bool {
	if o[i].ObjectMeta.CreationTimestamp.IsZero() && !o[j].ObjectMeta.CreationTimestamp.IsZero() {
		return false
	}
	if !o[i].ObjectMeta.CreationTimestamp.IsZero() && o[j].ObjectMeta.CreationTimestamp.IsZero() {
		return true
	}

	if o[i].ObjectMeta.CreationTimestamp.Equal(&o[j].ObjectMeta.CreationTimestamp) {
		return true
	}
	return o[i].ObjectMeta.CreationTimestamp.Before(&o[j].ObjectMeta.CreationTimestamp)
}
