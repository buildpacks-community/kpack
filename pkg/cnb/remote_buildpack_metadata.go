package cnb

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
)

type RemoteBuildpackInfo struct {
	BuildpackInfo v1alpha1.BuildpackInfo
	Layers        []buildpackLayer
}

func (i RemoteBuildpackInfo) Optional(optional bool) RemoteBuildpackRef {
	return RemoteBuildpackRef{
		BuildpackRef: v1alpha1.BuildpackRef{
			BuildpackInfo: i.BuildpackInfo,
			Optional:      optional,
		},
		Layers: i.Layers,
	}
}

type RemoteBuildpackRef struct {
	BuildpackRef v1alpha1.BuildpackRef
	Layers       []buildpackLayer
}

type buildpackLayer struct {
	v1Layer            v1.Layer
	BuildpackInfo      v1alpha1.BuildpackInfo
	BuildpackLayerInfo BuildpackLayerInfo
}
