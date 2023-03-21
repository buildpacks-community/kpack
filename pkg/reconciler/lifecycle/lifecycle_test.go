package lifecycle_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/controller"

	"github.com/pivotal/kpack/pkg/config"
	"github.com/pivotal/kpack/pkg/reconciler/lifecycle"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
)

func TestLifecycleReconciler(t *testing.T) {
	spec.Run(t, "Lifecycle Reconciler", testLifecycleReconciler)
}

func testLifecycleReconciler(t *testing.T, when spec.G, it spec.S) {

	var (
		fakeTracker        = &testhelpers.FakeTracker{}
		lifecycleImageRef  = "gcr.io/lifecycle@sha256:some-sha"
		serviceAccountName = "lifecycle-sa"
		namespace          = "kpack"
		key                = types.NamespacedName{Namespace: namespace, Name: config.LifecycleConfigName}
		lifecycleProvider  = &fakeLifecycleProvider{}
	)

	lifecycleConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.LifecycleConfigName,
			Namespace: namespace,
		},
		Data: map[string]string{
			config.LifecycleConfigKey:     lifecycleImageRef,
			"serviceAccountRef.name":      serviceAccountName,
			"serviceAccountRef.namespace": namespace,
		},
	}
	listers := testhelpers.NewListers([]runtime.Object{lifecycleConfigMap})
	k8sfakeClient := k8sfake.NewSimpleClientset(listers.GetKubeObjects()...)

	r := &lifecycle.Reconciler{
		Tracker:           fakeTracker,
		K8sClient:         k8sfakeClient,
		ConfigMapLister:   listers.GetConfigMapLister(),
		LifecycleProvider: lifecycleProvider,
	}

	when("Reconcile", func() {
		it("calls UpdateImage", func() {
			err := r.Reconcile(context.TODO(), key.String())
			require.NoError(t, err)
			require.Len(t, lifecycleProvider.Calls, 1)
			assert.Equal(t, lifecycleProvider.Calls[0], lifecycleConfigMap)
		})

		it("returns error if key is invalid", func() {
			err := r.Reconcile(context.TODO(), "my-namespace/fake/config-map")
			require.Error(t, err, "unexpected key")
			assert.False(t, controller.IsPermanentError(err))
		})

		it("returns error if config map does not exist", func() {
			err := r.Reconcile(context.TODO(), "my-namespace/fake-config-map")
			require.Error(t, err, "configmap \"fake-config-map\" not found")
			assert.False(t, controller.IsPermanentError(err))
		})

		it("returns error update image fails", func() {
			lifecycleProvider.returnsOnCall(fmt.Errorf("some update error"))
			err := r.Reconcile(context.TODO(), "my-namespace/fake-config-map")
			require.Error(t, err, "some update error")
			assert.False(t, controller.IsPermanentError(err))
		})
	})
}

type fakeLifecycleProvider struct {
	Calls []*corev1.ConfigMap
	error error
}

func (f *fakeLifecycleProvider) UpdateImage(cm *corev1.ConfigMap) error {
	f.Calls = append(f.Calls, cm)
	return f.error
}

func (f *fakeLifecycleProvider) returnsOnCall(err error) {
	f.error = err
}
