package cnb_test

import (
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestRemoteLifecycleReader(t *testing.T) {
	spec.Run(t, "Test Lifecycle Reader", testRemoteLifecycleReader)
}

func testRemoteLifecycleReader(t *testing.T, when spec.G, it spec.S) {
	when("Remote Lifecycle Reader", func() {
		const lifecycleTag = "gcr.io/image/lifecycle"

		var (
			fakeClient = registryfakes.NewFakeClient()

			expectedKeychain      = authn.NewMultiKeychain(authn.DefaultKeychain)
			remoteLifecycleReader = &cnb.RemoteLifecycleReader{
				RegistryClient: fakeClient,
			}
		)

		it("returns lifecycle metadata", func() {
			lifecycleImage := lifecycleImage(t, "some-version")

			fakeClient.AddImage(lifecycleTag, lifecycleImage, expectedKeychain)

			resolvedLifecycle, err := remoteLifecycleReader.Read(expectedKeychain, buildapi.ClusterLifecycleSpec{
				ImageSource: corev1alpha1.ImageSource{
					Image: lifecycleTag,
				},
			})
			require.NoError(t, err)

			assert.Equal(t, "gcr.io/image/lifecycle", resolvedLifecycle.Image.Image)
			assert.Equal(t, "some-version", resolvedLifecycle.Version)
			assert.Equal(
				t,
				buildapi.LifecycleAPI{
					BuildpackVersion: "0.7",
					PlatformVersion:  "0.7",
				},
				resolvedLifecycle.API,
			)
			assert.Equal(
				t,
				buildapi.LifecycleAPIs{
					Buildpack: buildapi.APIVersions{Supported: buildapi.APISet{"0.7", "0.8", "0.9", "0.10", "0.11"}},
					Platform:  buildapi.APIVersions{Supported: buildapi.APISet{"0.7", "0.8", "0.9", "0.10", "0.11", "0.12", "0.13"}},
				},
				resolvedLifecycle.APIs,
			)
		})
	})
}

func lifecycleImage(t *testing.T, version string) v1.Image {
	image, err := random.Image(10, 10)
	require.NoError(t, err)

	image, err = imagehelpers.SetStringLabels(
		image,
		map[string]string{
			"io.buildpacks.builder.metadata": fmt.Sprintf("{\"lifecycle\":{\"version\":\"%s\"},\"api\":{\"buildpack\":\"0.7\",\"platform\":\"0.7\"}}", version),
			"io.buildpacks.lifecycle.apis":   "{\"buildpack\":{\"deprecated\":[],\"supported\":[\"0.7\",\"0.8\",\"0.9\",\"0.10\",\"0.11\"]},\"platform\":{\"deprecated\":[],\"supported\":[\"0.7\",\"0.8\",\"0.9\",\"0.10\",\"0.11\",\"0.12\",\"0.13\"]}}",
		},
	)
	require.NoError(t, err)

	return image
}
