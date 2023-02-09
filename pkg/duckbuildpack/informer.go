package duckbuildpack

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	buildinformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha2"
	buildlisters "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
)

type DuckBuildpackInformer struct {
	BuildpackInformer        buildinformers.BuildpackInformer
	ClusterBuildpackInformer buildinformers.ClusterBuildpackInformer
}

func (di *DuckBuildpackInformer) AddBuildpackEventHandler(handler cache.ResourceEventHandler) {
	di.BuildpackInformer.Informer().AddEventHandler(handler)
}

func (di *DuckBuildpackInformer) AddClusterBuildpackEventHandler(handler cache.ResourceEventHandler) {
	di.ClusterBuildpackInformer.Informer().AddEventHandler(handler)
}

func (di *DuckBuildpackInformer) Lister() *DuckBuildpackLister {
	return &DuckBuildpackLister{
		BuildpackLister:        di.BuildpackInformer.Lister(),
		ClusterBuildpackLister: di.ClusterBuildpackInformer.Lister(),
	}
}

type DuckBuildpackLister struct {
	BuildpackLister        buildlisters.BuildpackLister
	ClusterBuildpackLister buildlisters.ClusterBuildpackLister
}

// Returns a Lister that will list Buildpacks and ClusterBuildpacks if
// namespace is specified. Otherwise only list ClusterBuildpacks
func (bl *DuckBuildpackLister) Namespace(namespace string) *DuckBuildpackNamespaceLister {
	return &DuckBuildpackNamespaceLister{
		DuckBuilderLister: bl,
		namespace:         namespace,
	}
}

type DuckBuildpackNamespaceLister struct {
	DuckBuilderLister *DuckBuildpackLister
	namespace         string
}

func (bl *DuckBuildpackNamespaceLister) List(selector labels.Selector) ([]*DuckBuildpack, error) {
	var bps []*DuckBuildpack
	if bl.namespace != metav1.NamespaceAll {
		bp, err := bl.DuckBuilderLister.BuildpackLister.Buildpacks(bl.namespace).List(selector)
		if err != nil {
			return nil, err
		}
		bps = append(bps, convertBuildpack(bp)...)
	}

	cbp, err := bl.DuckBuilderLister.ClusterBuildpackLister.List(selector)
	if err != nil {
		return nil, err
	}
	bps = append(bps, convertClusterBuildpack(cbp)...)

	return bps, nil
}

func convertBuildpack(buildpacks []*buildapi.Buildpack) []*DuckBuildpack {
	bps := make([]*DuckBuildpack, len(buildpacks))
	for i, bp := range buildpacks {
		var serviceAccount *corev1.ObjectReference
		if bp.Spec.ServiceAccountName != "" {
			serviceAccount = &corev1.ObjectReference{
				Name:      bp.Spec.ServiceAccountName,
				Namespace: bp.Namespace,
			}
		}

		bps[i] = &DuckBuildpack{
			TypeMeta:   bp.TypeMeta,
			ObjectMeta: bp.ObjectMeta,
			Spec: DuckBuildpackSpec{
				ServiceAccountRef: serviceAccount,
			},
			Status: bp.Status,
		}
	}
	return bps
}

func convertClusterBuildpack(clusterBuildpacks []*buildapi.ClusterBuildpack) []*DuckBuildpack {
	cbps := make([]*DuckBuildpack, len(clusterBuildpacks))
	for i, cbp := range clusterBuildpacks {
		cbps[i] = &DuckBuildpack{
			TypeMeta:   cbp.TypeMeta,
			ObjectMeta: cbp.ObjectMeta,
			Status:     cbp.Status,
			Spec: DuckBuildpackSpec{
				ServiceAccountRef: cbp.Spec.ServiceAccountRef,
			},
		}
	}
	return cbps
}
