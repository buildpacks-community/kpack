package config

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestProvider(t *testing.T) {
	spec.Run(t, "LifecycleProvider", testProvider)
}

func testProvider(t *testing.T, when spec.G, it spec.S) {
	var (
		client          = registryfakes.NewFakeClient()
		keychain        = authn.NewMultiKeychain(authn.DefaultKeychain)
		lifecycleImgRef = "some-image"
		lifecycleImg    v1.Image
		callBack        *fakeCallback
		err             error
		keychainFactory = &registryfakes.FakeKeychainFactory{}
		p               *LifecycleProvider
	)

	it.Before(func() {
		lifecycleImg, err = random.Image(10, int64(1))
		require.NoError(t, err)

		keychainFactory.AddKeychainForSecretRef(t, registry.SecretRef{Namespace: "some-service-account-namespace", ServiceAccount: "some-service-account"}, keychain)
		client.AddImage(lifecycleImgRef, lifecycleImg, keychain)

		p = NewLifecycleProvider(client, keychainFactory)
		callBack = &fakeCallback{}
		p.AddEventHandler(callBack.callBack)
	})

	it("sets and gets the image from the ConfigMap and calls handlers", func() {
		cfg := &corev1.ConfigMap{
			Data: map[string]string{"image": "some-image", "serviceAccountRef.name": "some-service-account", "serviceAccountRef.namespace": "some-service-account-namespace"},
		}

		p.UpdateImage(cfg)
		img, err := p.GetImage()
		require.NoError(t, err)
		require.Equal(t, lifecycleImg, img)
		require.Equal(t, callBack.called, 1)
	})

	it("does not call handlers when the lifecycle image has not changed", func() {
		cfg := &corev1.ConfigMap{
			Data: map[string]string{"image": "some-image", "serviceAccountRef.name": "some-service-account", "serviceAccountRef.namespace": "some-service-account-namespace"},
		}

		p.UpdateImage(cfg)
		p.UpdateImage(cfg)
		img, err := p.GetImage()
		require.NoError(t, err)
		require.Equal(t, lifecycleImg, img)
		require.Equal(t, callBack.called, 1)
	})

	it("updates after an error", func() {
		cfg := &corev1.ConfigMap{
			Data: map[string]string{"image": "invalid"},
		}
		p.UpdateImage(cfg)
		_, err := p.GetImage()
		require.Error(t, err)

		cfg = &corev1.ConfigMap{
			Data: map[string]string{"image": "some-image", "serviceAccountRef.name": "some-service-account", "serviceAccountRef.namespace": "some-service-account-namespace"},
		}
		p.UpdateImage(cfg)
		img, err := p.GetImage()
		require.NoError(t, err)
		require.Equal(t, lifecycleImg, img)
	})

	it("errors when the image key is invalid and calls handlers", func() {
		cfg := &corev1.ConfigMap{
			Data: map[string]string{"invalid": "some-other-image"},
		}

		p.UpdateImage(cfg)
		_, err := p.GetImage()
		require.EqualError(t, err, "lifecycle-image config invalid")
		require.Equal(t, callBack.called, 1)
	})

	it("errors when it has not loaded an image yet", func() {
		p = &LifecycleProvider{}
		_, err := p.GetImage()
		require.EqualError(t, err, "lifecycle image has not been loaded")
	})
}

type fakeCallback struct {
	called int
}

func (cb *fakeCallback) callBack() {
	cb.called++
}
