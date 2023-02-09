package duckbuildpack

import (
	"sync"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/client/informers/externalversions"
)

func TestDuckBuildpackInformer(t *testing.T) {
	spec.Run(t, "TestDuckBuildpackInformer", testDuckBuildpackInformer)
}

func testDuckBuildpackInformer(t *testing.T, when spec.G, it spec.S) {
	const (
		testNamespace        = "some-other-namespace"
		buildpackName        = "some-buildpack"
		clusterBuildpackName = "some-cluster-buildpack"
	)

	var (
		stopCh = make(chan struct{})

		buildpack = &buildapi.Buildpack{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1alpha2",
				Kind:       "Buildpack",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      buildpackName,
				Namespace: testNamespace,
			},
			Spec: buildapi.BuildpackSpec{},
			Status: buildapi.BuildpackStatus{
				Buildpacks: []corev1alpha1.BuildpackStatus{
					{
						BuildpackInfo: corev1alpha1.BuildpackInfo{
							Id:      "io.buildpack.engine",
							Version: "1.0.0",
						},
					},
				},
			},
		}

		clusterBuildpack = &buildapi.ClusterBuildpack{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1alpha2",
				Kind:       "ClusterBuildpack",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterBuildpackName,
			},
			Spec: buildapi.ClusterBuildpackSpec{},
			Status: buildapi.BuildpackStatus{
				Buildpacks: []corev1alpha1.BuildpackStatus{
					{
						BuildpackInfo: corev1alpha1.BuildpackInfo{
							Id:      "io.buildpack.package-manager",
							Version: "1.0.0",
						},
					},
				},
			},
		}
	)

	client := fake.NewSimpleClientset(
		buildpack,
		clusterBuildpack,
	)

	factory := externalversions.NewSharedInformerFactory(client, 10*time.Hour)

	subject := DuckBuildpackInformer{
		BuildpackInformer:        factory.Kpack().V1alpha2().Buildpacks(),
		ClusterBuildpackInformer: factory.Kpack().V1alpha2().ClusterBuildpacks(),
	}
	duckBuildpackLister := subject.Lister()
	factory.Start(stopCh)

	factory.WaitForCacheSync(stopCh)

	it.After(func() {
		close(stopCh)
	})

	when("#Lister", func() {
		it("returns Buildpacks and ClusterBuildpacks for a namespace", func() {
			duckBuildpacks, err := duckBuildpackLister.Namespace(testNamespace).List(labels.Everything())
			require.NoError(t, err)

			require.Len(t, duckBuildpacks, 2)
			require.Equal(t, buildpack.TypeMeta, duckBuildpacks[0].TypeMeta)
			require.Equal(t, buildpack.ObjectMeta, duckBuildpacks[0].ObjectMeta)
		})

		it("returns ClusterBuildpacks when using all namespaces", func() {
			duckBuildpacks, err := duckBuildpackLister.Namespace(v1.NamespaceAll).List(labels.Everything())
			require.NoError(t, err)

			require.Len(t, duckBuildpacks, 1)
			require.Equal(t, clusterBuildpack.TypeMeta, duckBuildpacks[0].TypeMeta)
			require.Equal(t, clusterBuildpack.ObjectMeta, duckBuildpacks[0].ObjectMeta)
		})

		it("returns cluster buildpacks when namespace doesnt' match", func() {
			duckBuildpacks, err := duckBuildpackLister.Namespace(v1.NamespaceAll).List(labels.Everything())
			require.NoError(t, err)

			require.Len(t, duckBuildpacks, 1)
			require.Equal(t, clusterBuildpack.TypeMeta, duckBuildpacks[0].TypeMeta)
			require.Equal(t, clusterBuildpack.ObjectMeta, duckBuildpacks[0].ObjectMeta)
		})
	})

	when("#AddBuildpackEventHandler", func() {
		it("adds the event handler to  buildpacks's informer", func() {
			testHandler := &testHandler{}
			subject.AddBuildpackEventHandler(testHandler)

			assert.Eventually(t, func() bool {
				return len(testHandler.added) == 1
			}, 5*time.Second, time.Millisecond)

			assert.Contains(t, testHandler.added, buildpack)
		})
	})

	when("#AddClusterBuildpackEventHandler", func() {
		it("adds the event handler to cluster buildpack's informer", func() {
			testHandler := &testHandler{}
			subject.AddClusterBuildpackEventHandler(testHandler)

			assert.Eventually(t, func() bool {
				return len(testHandler.added) == 1
			}, 5*time.Second, time.Millisecond)

			assert.Contains(t, testHandler.added, clusterBuildpack)
		})
	})
}

type testHandler struct {
	added []interface{}
	sync.Mutex
}

func (t *testHandler) OnAdd(obj interface{}) {
	t.Lock()
	defer t.Unlock()
	t.added = append(t.added, obj)
}

func (t *testHandler) OnUpdate(oldObj, newObj interface{}) {
}

func (t *testHandler) OnDelete(obj interface{}) {
}
