package config

//
//import (
//	"testing"
//
//	"github.com/google/go-containerregistry/pkg/authn"
//	v1 "github.com/google/go-containerregistry/pkg/v1"
//	"github.com/google/go-containerregistry/pkg/v1/empty"
//	"github.com/google/go-containerregistry/pkg/v1/mutate"
//	"github.com/google/go-containerregistry/pkg/v1/random"
//	"github.com/google/go-containerregistry/pkg/v1/types"
//	"github.com/sclevine/spec"
//	"github.com/stretchr/testify/require"
//	corev1 "k8s.io/api/core/v1"
//
//	"github.com/pivotal/kpack/pkg/cnb"
//	"github.com/pivotal/kpack/pkg/registry"
//	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
//	"github.com/pivotal/kpack/pkg/registry/registryfakes"
//)
//
//func TestKeychainFactoryProvider(t *testing.T) {
//	spec.Run(t, "KeychainFactoryProvider", testKeychainFactoryProvider)
//}
//
//func testKeychainFactoryProvider(t *testing.T, when spec.G, it spec.S) {
//	var (
//		lifecycleMetadata = cnb.LifecycleMetadata{
//			LifecycleInfo: cnb.LifecycleInfo{
//				Version: "0.5.0",
//			},
//			API: cnb.LifecycleAPI{
//				BuildpackVersion: "0.2",
//				PlatformVersion:  "0.1",
//			},
//			APIs: cnb.LifecycleAPIs{
//				Buildpack: cnb.APIVersions{
//					Deprecated: []string{"0.2"},
//					Supported:  []string{"0.3"},
//				},
//				Platform: cnb.APIVersions{
//					Deprecated: []string{"0.3"},
//					Supported:  []string{"0.4"},
//				},
//			},
//		}
//		client          = registryfakes.NewFakeClient()
//		keychain        = authn.NewMultiKeychain(authn.DefaultKeychain)
//		lifecycleImgRef = "some-image"
//		lifecycleImg    v1.Image
//		linuxLayer      v1.Layer
//		windowsLayer    v1.Layer
//		callBack        *fakeCallback
//		keychainFactory = &registryfakes.FakeKeychainFactory{}
//		p               *LifecycleProvider
//	)
//
//	it.Before(func() {
//		linuxLayer = testLayer(t)
//		windowsLayer = testLayer(t)
//		lifecycleImg = generateLifecycleImage(t, lifecycleMetadata, linuxLayer, windowsLayer)
//
//		keychainFactory.AddKeychainForSecretRef(t, registry.SecretRef{Namespace: "some-service-account-namespace", ServiceAccount: "some-service-account"}, keychain)
//		client.AddImage(lifecycleImgRef, lifecycleImg, keychain)
//		client.AddImage("some-other-lifecycle-image", generateLifecycleImage(t, lifecycleMetadata, testLayer(t), testLayer(t)), keychain)
//
//		p = NewLifecycleProvider(client, keychainFactory)
//		callBack = &fakeCallback{}
//		p.AddEventHandler(callBack.callBack)
//	})
//
//	when("Update Keychain Factory is called with a ConfigMap", func() {
//
//	})
//}
//
//
//
