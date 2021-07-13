package testhelpers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/reconciler/testing"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	buildlisters "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/duckbuilder"
)

var clientSetSchemes = []func(*runtime.Scheme) error{
	fake.AddToScheme,
	fakekubeclientset.AddToScheme,
}

type Listers struct {
	sorter testing.ObjectSorter
}

func NewListers(objs []runtime.Object) Listers {
	scheme := runtime.NewScheme()

	for _, addTo := range clientSetSchemes {
		addTo(scheme)
	}

	ls := Listers{
		sorter: testing.NewObjectSorter(scheme),
	}

	ls.sorter.AddObjects(objs...)

	return ls
}

func (l *Listers) indexerFor(obj runtime.Object) cache.Indexer {
	return l.sorter.IndexerForObjectType(obj)
}

func (l *Listers) BuildServiceObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fake.AddToScheme)
}

func (l *Listers) GetKubeObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakekubeclientset.AddToScheme)
}

func (l *Listers) GetImageLister() buildlisters.ImageLister {
	return buildlisters.NewImageLister(l.indexerFor(&buildapi.Image{}))
}

func (l *Listers) GetBuildLister() buildlisters.BuildLister {
	return buildlisters.NewBuildLister(l.indexerFor(&buildapi.Build{}))
}

func (l *Listers) GetBuilderLister() buildlisters.BuilderLister {
	return buildlisters.NewBuilderLister(l.indexerFor(&buildapi.Builder{}))
}

func (l *Listers) GetClusterBuilderLister() buildlisters.ClusterBuilderLister {
	return buildlisters.NewClusterBuilderLister(l.indexerFor(&buildapi.ClusterBuilder{}))
}

func (l *Listers) GetClusterStoreLister() buildlisters.ClusterStoreLister {
	return buildlisters.NewClusterStoreLister(l.indexerFor(&buildapi.ClusterStore{}))
}

func (l *Listers) GetClusterStackLister() buildlisters.ClusterStackLister {
	return buildlisters.NewClusterStackLister(l.indexerFor(&buildapi.ClusterStack{}))
}

func (l *Listers) GetSourceResolverLister() buildlisters.SourceResolverLister {
	return buildlisters.NewSourceResolverLister(l.indexerFor(&buildapi.SourceResolver{}))
}

func (l *Listers) GetPersistentVolumeClaimLister() corev1listers.PersistentVolumeClaimLister {
	return corev1listers.NewPersistentVolumeClaimLister(l.indexerFor(&corev1.PersistentVolumeClaim{}))
}

func (l *Listers) GetPodLister() corev1listers.PodLister {
	return corev1listers.NewPodLister(l.indexerFor(&corev1.Pod{}))
}

func (l *Listers) GetDuckBuilderLister() *duckbuilder.DuckBuilderLister {
	return &duckbuilder.DuckBuilderLister{
		BuilderLister:        l.GetBuilderLister(),
		ClusterBuilderLister: l.GetClusterBuilderLister(),
	}
}
