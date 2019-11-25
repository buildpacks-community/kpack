package cnb

import v1 "github.com/google/go-containerregistry/pkg/v1"

type RemoteBuildpackInfo struct {
	BuildpackInfo BuildpackInfo
	Layers        []buildpackLayer
}

func (i RemoteBuildpackInfo) Optional(optional bool) RemoteBuildpackRef {
	return RemoteBuildpackRef{
		BuildpackRef: BuildpackRef{
			BuildpackInfo: i.BuildpackInfo,
			Optional:      optional,
		},
		Layers: i.Layers,
	}
}

type RemoteBuildpackRef struct {
	BuildpackRef BuildpackRef
	Layers       []buildpackLayer
}

type buildpackLayer struct {
	v1Layer       v1.Layer
	BuildpackInfo BuildpackInfo
	Order         Order
}
