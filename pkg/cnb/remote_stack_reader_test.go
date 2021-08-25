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
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestRemoteStackReader(t *testing.T) {
	spec.Run(t, "Test Stack Reader", testRemoteStackReader)
}

func testRemoteStackReader(t *testing.T, when spec.G, it spec.S) {
	when("Remote Stack Reader", func() {
		const (
			runTag   = "gcr.io/image/run"
			buildTag = "gcr.io/image/build"
			stackId  = "org.some.stack"
		)

		var (
			fakeClient = registryfakes.NewFakeClient()

			expectedKeychain  = authn.NewMultiKeychain(authn.DefaultKeychain)
			remoteStackReader = &cnb.RemoteStackReader{
				RegistryClient: fakeClient,
			}
		)

		it("returns resolves images and mixins of stack images", func() {
			runImage := runImage(t, stackId, []string{"shared-mixin", "run:run-mixin"})
			buildImage := buildImage(t, stackId, []string{"shared-mixin", "build:build-mixin"})

			fakeClient.AddImage(runTag, runImage, expectedKeychain)
			fakeClient.AddImage(buildTag, buildImage, expectedKeychain)

			resolvedStack, err := remoteStackReader.Read(expectedKeychain, buildapi.ClusterStackSpec{
				Id: "org.some.stack",
				BuildImage: buildapi.ClusterStackSpecImage{
					Image: buildTag,
				},
				RunImage: buildapi.ClusterStackSpecImage{
					Image: runTag,
				},
			})
			require.NoError(t, err)

			runDigest, err := runImage.Digest()
			require.NoError(t, err)

			buildDigest, err := buildImage.Digest()
			require.NoError(t, err)

			assert.Equal(t, buildapi.ResolvedClusterStack{
				Id: stackId,
				BuildImage: buildapi.ClusterStackStatusImage{
					LatestImage: fmt.Sprintf("%s@%s", buildTag, buildDigest),
					Image:       buildTag,
				},
				RunImage: buildapi.ClusterStackStatusImage{
					LatestImage: fmt.Sprintf("%s@%s", runTag, runDigest),
					Image:       runTag,
				},
				Mixins:  []string{"shared-mixin", "build:build-mixin", "run:run-mixin"},
				UserID:  1000,
				GroupID: 2000,
			}, resolvedStack)

		})

		when("invalid", func() {
			it("returns error if stack id does not match run image", func() {
				runImage := runImage(t, "something.else", nil)
				buildImage := buildImage(t, stackId, nil)

				fakeClient.AddImage(runTag, runImage, expectedKeychain)
				fakeClient.AddImage(buildTag, buildImage, expectedKeychain)

				_, err := remoteStackReader.Read(expectedKeychain, buildapi.ClusterStackSpec{
					Id: "org.some.stack",
					BuildImage: buildapi.ClusterStackSpecImage{
						Image: buildTag,
					},
					RunImage: buildapi.ClusterStackSpecImage{
						Image: runTag,
					},
				})
				require.EqualError(t, err, "invalid stack images. expected stack: org.some.stack, build image stack: org.some.stack, run image stack: something.else")
			})

			it("returns error if stack id does not match build image", func() {
				runImage := runImage(t, stackId, nil)
				buildImage := buildImage(t, "something.wrong", nil)

				fakeClient.AddImage(runTag, runImage, expectedKeychain)
				fakeClient.AddImage(buildTag, buildImage, expectedKeychain)

				_, err := remoteStackReader.Read(expectedKeychain, buildapi.ClusterStackSpec{
					Id: "org.some.stack",
					BuildImage: buildapi.ClusterStackSpecImage{
						Image: buildTag,
					},
					RunImage: buildapi.ClusterStackSpecImage{
						Image: runTag,
					},
				})
				require.EqualError(t, err, "invalid stack images. expected stack: org.some.stack, build image stack: something.wrong, run image stack: org.some.stack")
			})

			it("returns error if run image is missing mixins", func() {
				buildImage := buildImage(t, stackId, []string{"some-required-mixin", "some-required-mixin2"})
				runImage := runImage(t, stackId, nil)

				fakeClient.AddImage(runTag, runImage, expectedKeychain)
				fakeClient.AddImage(buildTag, buildImage, expectedKeychain)

				_, err := remoteStackReader.Read(expectedKeychain, buildapi.ClusterStackSpec{
					Id: "org.some.stack",
					BuildImage: buildapi.ClusterStackSpecImage{
						Image: buildTag,
					},
					RunImage: buildapi.ClusterStackSpecImage{
						Image: runTag,
					},
				})
				require.EqualError(t, err, "runImage missing required mixin(s): some-required-mixin, some-required-mixin2")
			})

			it("returns error if run image has build mixins", func() {
				runImage := runImage(t, stackId, []string{"build:invalid", "build:invalid2"})
				buildImage := buildImage(t, stackId, nil)

				fakeClient.AddImage(runTag, runImage, expectedKeychain)
				fakeClient.AddImage(buildTag, buildImage, expectedKeychain)

				_, err := remoteStackReader.Read(expectedKeychain, buildapi.ClusterStackSpec{
					Id: "org.some.stack",
					BuildImage: buildapi.ClusterStackSpecImage{
						Image: buildTag,
					},
					RunImage: buildapi.ClusterStackSpecImage{
						Image: runTag,
					},
				})
				require.EqualError(t, err, "run image contains build-only mixin(s): build:invalid, build:invalid2")
			})

			it("returns error if build image has run mixins", func() {
				buildImage := buildImage(t, stackId, []string{"run:invalid", "run:invalid2"})
				runImage := runImage(t, stackId, nil)

				fakeClient.AddImage(runTag, runImage, expectedKeychain)
				fakeClient.AddImage(buildTag, buildImage, expectedKeychain)

				_, err := remoteStackReader.Read(expectedKeychain, buildapi.ClusterStackSpec{
					Id: "org.some.stack",
					BuildImage: buildapi.ClusterStackSpecImage{
						Image: buildTag,
					},
					RunImage: buildapi.ClusterStackSpecImage{
						Image: runTag,
					},
				})
				require.EqualError(t, err, "build image contains run-only mixin(s): run:invalid, run:invalid2")
			})

			it("returns error if build image does not have required env vars", func() {
				runImage := runImage(t, stackId, nil)

				fakeClient.AddImage(runTag, runImage, expectedKeychain)

				_, err := remoteStackReader.Read(expectedKeychain, buildapi.ClusterStackSpec{
					Id: "org.some.stack",
					BuildImage: buildapi.ClusterStackSpecImage{
						Image: runTag,
					},
					RunImage: buildapi.ClusterStackSpecImage{
						Image: runTag,
					},
				})
				require.EqualError(t, err, "validating build image: ENV CNB_USER_ID not found")
			})
		})
	})
}

func runImage(t *testing.T, stackId string, mixins []string) v1.Image {
	runImage, err := random.Image(10, 10)
	require.NoError(t, err)

	runImage, err = imagehelpers.SetStringLabel(runImage, "io.buildpacks.stack.id", stackId)
	require.NoError(t, err)

	if len(mixins) != 0 {
		runImage, err = imagehelpers.SetLabels(runImage, map[string]interface{}{
			"io.buildpacks.stack.mixins": mixins,
		})
		require.NoError(t, err)
	}

	return runImage
}

func buildImage(t *testing.T, stackId string, mixins []string) v1.Image {
	runImage, err := random.Image(10, 10)
	require.NoError(t, err)

	runImage, err = imagehelpers.SetEnv(runImage, "CNB_USER_ID", "1000")
	require.NoError(t, err)
	runImage, err = imagehelpers.SetEnv(runImage, "CNB_GROUP_ID", "2000")
	require.NoError(t, err)

	runImage, err = imagehelpers.SetStringLabel(runImage, "io.buildpacks.stack.id", stackId)
	require.NoError(t, err)

	if len(mixins) != 0 {
		runImage, err = imagehelpers.SetLabels(runImage, map[string]interface{}{
			"io.buildpacks.stack.mixins": mixins,
		})
		require.NoError(t, err)
	}

	return runImage
}
