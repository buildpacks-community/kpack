package config

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestProvider(t *testing.T) {
	spec.Run(t, "LifecycleProvider", testProvider)
}

func testProvider(t *testing.T, when spec.G, it spec.S) {
	var (
		lifecycleMetadata = cnb.LifecycleMetadata{
			LifecycleInfo: cnb.LifecycleInfo{
				Version: "0.5.0",
			},
			API: cnb.LifecycleAPI{
				BuildpackVersion: "0.2",
				PlatformVersion:  "0.1",
			},
			APIs: cnb.LifecycleAPIs{
				Buildpack: cnb.APIVersions{
					Deprecated: []string{"0.2"},
					Supported:  []string{"0.3"},
				},
				Platform: cnb.APIVersions{
					Deprecated: []string{"0.3"},
					Supported:  []string{"0.4"},
				},
			},
		}
		client          = registryfakes.NewFakeClient()
		keychain        = authn.NewMultiKeychain(authn.DefaultKeychain)
		lifecycleImgRef = "some-image"
		lifecycleImg    v1.Image
		linuxLayer      v1.Layer
		windowsLayer    v1.Layer
		callBack        *fakeCallback
		keychainFactory = &registryfakes.FakeKeychainFactory{}
		p               *LifecycleProvider
	)

	it.Before(func() {
		linuxLayer = testLayer(t)
		windowsLayer = testLayer(t)
		lifecycleImg = generateLifecycleImage(t, lifecycleMetadata, linuxLayer, windowsLayer)

		keychainFactory.AddKeychainForSecretRef(t, registry.SecretRef{Namespace: "some-service-account-namespace", ServiceAccount: "some-service-account"}, keychain)
		client.AddImage(lifecycleImgRef, lifecycleImg, keychain)
		client.AddImage("some-other-lifecycle-image", generateLifecycleImage(t, lifecycleMetadata, testLayer(t), testLayer(t)), keychain)

		p = NewLifecycleProvider(client, keychainFactory)
		callBack = &fakeCallback{}
		p.AddEventHandler(callBack.callBack)
	})

	when("Update Image is called with a ConfigMap", func() {
		it("calls all Handlers", func() {
			require.Equal(t, callBack.called, 0)

			p.UpdateImage(&corev1.ConfigMap{
				Data: map[string]string{"image": lifecycleImgRef, "serviceAccountRef.name": "some-service-account", "serviceAccountRef.namespace": "some-service-account-namespace"},
			})
			require.Equal(t, callBack.called, 1)

			p.UpdateImage(&corev1.ConfigMap{
				Data: map[string]string{"image": "some-other-lifecycle-image", "serviceAccountRef.name": "some-service-account", "serviceAccountRef.namespace": "some-service-account-namespace"},
			})
			require.Equal(t, callBack.called, 2)
		})

		it("does not call Handlers when lifecycle image is not updated", func() {
			require.Equal(t, callBack.called, 0)

			p.UpdateImage(&corev1.ConfigMap{
				Data: map[string]string{"image": lifecycleImgRef, "serviceAccountRef.name": "some-service-account", "serviceAccountRef.namespace": "some-service-account-namespace"},
			})
			require.Equal(t, callBack.called, 1)

			p.UpdateImage(&corev1.ConfigMap{
				Data: map[string]string{"image": lifecycleImgRef, "serviceAccountRef.name": "some-service-account", "serviceAccountRef.namespace": "some-service-account-namespace"},
			})
			require.Equal(t, callBack.called, 1)
		})

		it("does not call Handlers when updated lifecycle image is invalid", func() {
			require.Equal(t, callBack.called, 0)

			p.UpdateImage(&corev1.ConfigMap{
				Data: map[string]string{"image": lifecycleImgRef, "serviceAccountRef.name": "some-service-account", "serviceAccountRef.namespace": "some-service-account-namespace"},
			})
			require.Equal(t, callBack.called, 1)

			p.UpdateImage(&corev1.ConfigMap{
				Data: map[string]string{"image": "some-invalid-image", "serviceAccountRef.name": "some-service-account", "serviceAccountRef.namespace": "some-service-account-namespace"},
			})
			require.Equal(t, callBack.called, 1)

			_, _, err := p.LayerForOS("linux")
			require.Error(t, err)
		})
	})

	when("LayerForOS()", func() {
		it.Before(func() {
			p.UpdateImage(&corev1.ConfigMap{
				Data: map[string]string{"image": lifecycleImgRef, "serviceAccountRef.name": "some-service-account", "serviceAccountRef.namespace": "some-service-account-namespace"},
			})
		})

		it("returns the linux layer as a lazy layer", func() {
			layer, readMetadata, err := p.LayerForOS("linux")
			require.NoError(t, err)
			require.Equal(t, readMetadata, lifecycleMetadata)

			expectedDigest, err := linuxLayer.Digest()
			require.NoError(t, err)

			expectedDiffID, err := linuxLayer.DiffID()
			require.NoError(t, err)

			expectedSize, err := linuxLayer.Size()
			require.NoError(t, err)

			expectedLayer, err := imagehelpers.NewLazyMountableLayer(imagehelpers.LazyMountableLayerArgs{
				Digest:   expectedDigest.String(),
				DiffId:   expectedDiffID.String(),
				Image:    lifecycleImgRef,
				Size:     expectedSize,
				Keychain: keychain,
			})
			require.NoError(t, err)

			require.Equal(t, expectedLayer, layer)

		})

		it("returns the windows layer as a lazy layer", func() {
			layer, readMetadata, err := p.LayerForOS("windows")
			require.NoError(t, err)
			require.Equal(t, readMetadata, lifecycleMetadata)

			expectedDigest, err := windowsLayer.Digest()
			require.NoError(t, err)

			expectedDiffID, err := windowsLayer.DiffID()
			require.NoError(t, err)

			expectedSize, err := windowsLayer.Size()
			require.NoError(t, err)

			expectedLayer, err := imagehelpers.NewLazyMountableLayer(imagehelpers.LazyMountableLayerArgs{
				Digest:   expectedDigest.String(),
				DiffId:   expectedDiffID.String(),
				Image:    lifecycleImgRef,
				Size:     expectedSize,
				Keychain: keychain,
			})
			require.NoError(t, err)

			require.Equal(t, expectedLayer, layer)
		})

		it("returns error on invalid os", func() {
			_, _, err := p.LayerForOS("kpack-invalid-test-os")
			require.EqualError(t, err, "unrecognized os kpack-invalid-test-os")
		})
	})
}

type fakeCallback struct {
	called int
}

func (cb *fakeCallback) callBack() {
	cb.called++
}

func generateLifecycleImage(t *testing.T, metadata cnb.LifecycleMetadata, linuxLifecycle, windowsLifecycle v1.Layer) v1.Image {
	lifecycleImg, err := mutate.AppendLayers(empty.Image, linuxLifecycle, windowsLifecycle)
	require.NoError(t, err)

	linuxDiffID, err := linuxLifecycle.DiffID()
	require.NoError(t, err)

	lifecycleImg, err = imagehelpers.SetStringLabel(lifecycleImg, "linux", linuxDiffID.String())
	require.NoError(t, err)

	windowsDiffId, err := windowsLifecycle.DiffID()
	require.NoError(t, err)

	lifecycleImg, err = imagehelpers.SetStringLabel(lifecycleImg, "windows", windowsDiffId.String())
	require.NoError(t, err)

	lifecycleImg, err = imagehelpers.SetLabels(lifecycleImg, map[string]interface{}{
		lifecycleMetadataLabel: metadata,
	})
	require.NoError(t, err)

	return lifecycleImg
}

func testLayer(t *testing.T) v1.Layer {
	linuxLifecycle, err := random.Layer(10, types.DockerLayer)
	require.NoError(t, err)
	return linuxLifecycle
}
