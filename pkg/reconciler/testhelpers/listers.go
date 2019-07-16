package testhelpers

import (
	"github.com/knative/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	v1alpha1Listers "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
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

func (l *Listers) GetImageLister() v1alpha1Listers.ImageLister {
	return v1alpha1Listers.NewImageLister(l.indexerFor(&v1alpha1.Image{}))
}

func (l *Listers) GetBuildLister() v1alpha1Listers.BuildLister {
	return v1alpha1Listers.NewBuildLister(l.indexerFor(&v1alpha1.Build{}))
}

func (l *Listers) GetBuilderLister() v1alpha1Listers.BuilderLister {
	return v1alpha1Listers.NewBuilderLister(l.indexerFor(&v1alpha1.Builder{}))
}

func (l *Listers) GetSourceResolverLister() v1alpha1Listers.SourceResolverLister {
	return v1alpha1Listers.NewSourceResolverLister(l.indexerFor(&v1alpha1.SourceResolver{}))
}

func (l *Listers) GetPersistentVolumeClaimLister() corev1listers.PersistentVolumeClaimLister {
	return corev1listers.NewPersistentVolumeClaimLister(l.indexerFor(&corev1.PersistentVolumeClaim{}))
}
