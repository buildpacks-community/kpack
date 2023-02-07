package cnb

import (
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry"
	k8sv1 "k8s.io/api/core/v1"
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

func (r RemoteBuildpackRef) buildpackRef() corev1alpha1.BuildpackRef {
	return corev1alpha1.BuildpackRef{
		BuildpackInfo: r.DescriptiveBuildpackInfo.BuildpackInfo,
		Optional:      r.Optional,
	}
}

type buildpackLayer struct {
	v1Layer            ggcrv1.Layer
	BuildpackInfo      DescriptiveBuildpackInfo
	BuildpackLayerInfo BuildpackLayerInfo
}

type K8sRemoteBuildpack struct {
	Buildpack corev1alpha1.BuildpackStatus
	SecretRef registry.SecretRef
	source    k8sv1.ObjectReference
}
