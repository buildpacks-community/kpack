package cnb

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

type RemoteBuildpackInfo struct {
	BuildpackInfo DescriptiveBuildpackInfo
	Layers        []buildpackLayer
}

func (i RemoteBuildpackInfo) Optional(optional bool) RemoteBuildpackRef {
	return RemoteBuildpackRef{
		DescriptiveBuildpackInfo: i.BuildpackInfo,
		Optional:                 optional,
		Layers:                   i.Layers,
	}
}

type RemoteBuildpackRef struct {
	DescriptiveBuildpackInfo DescriptiveBuildpackInfo
	Optional                 bool
	Layers                   []buildpackLayer
}

func (r RemoteBuildpackRef) buildpackRef() v1alpha1.BuildpackRef {
	return v1alpha1.BuildpackRef{
		BuildpackInfo: r.DescriptiveBuildpackInfo.BuildpackInfo,
		Optional:      r.Optional,
	}
}

type buildpackLayer struct {
	v1Layer            v1.Layer
	BuildpackInfo      DescriptiveBuildpackInfo
	BuildpackLayerInfo BuildpackLayerInfo
}
