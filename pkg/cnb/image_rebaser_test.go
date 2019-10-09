package cnb_test

import (
	"context"
	"testing"

	"github.com/buildpack/imgutil/fakes"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/cnb/cnbfakes"
	"github.com/pivotal/kpack/pkg/registry"
)

func TestImageRebase(t *testing.T) {
	spec.Run(t, "ImageRebaser", testImageRebaser)
}

func testImageRebaser(t *testing.T, when spec.G, it spec.S) {
	var (
		imgRebaser             cnb.ImageRebaser
		fakeRemoteImageFactory = &cnbfakes.FakeRemoteImageUtilFactory{}
	)

	when("#Rebase", func() {
		additionalTags := []string{"other-tag"}
		it("invokes Rebase on the previously built image", func() {
			pullSecrets := []v12.LocalObjectReference{
				{
					Name: "pullSecret1",
				},
				{
					Name: "pullSecret2",
				},
			}
			build := &v1alpha1.Build{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testBuild",
					Namespace: "testNamespace",
				},
				Spec: v1alpha1.BuildSpec{
					Tags: additionalTags,
					Builder: v1alpha1.BuildBuilderSpec{
						Image:            "testbuilder/builder",
						ImagePullSecrets: pullSecrets,
					},
					ServiceAccount: "testServiceAccount",
					LastBuild:      v1alpha1.LastBuild{Image: "testimage/app"},
				},
			}

			builderImage := fakes.NewImage("testbuilder/builder", "293847toplayer", &fakeImageIdentifier{identifier: "builder"})
			err := builderImage.SetLabel("io.buildpacks.builder.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}], "stack": { "runImage": { "image": "foo.io/run:basecnb" }}}`)
			require.NoError(t, err)
			fakeRemoteImageFactory.NewRemoteReturnsOnCall(0, builderImage, nil)

			appImage := fakes.NewImage("testimage/app", "980723452toplayer", &fakeImageIdentifier{identifier: "appimage"})
			err = appImage.SetLabel("io.buildpacks.lifecycle.metadata", `{"runImage":{"topLayer":"sha256:719f3f610dade1fdf5b4b2473aea0c6b1317497cf20691ab6d184a9b2fa5c409","reference":"localhost:5000/node@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0"},"stack":{"runImage":{"image":"cloudfoundry/run:full-cnb"}}}`)
			require.NoError(t, err)
			err = appImage.SetLabel("io.buildpacks.stack.id", "io.buildpacks.stacks.bionic")
			require.NoError(t, err)
			fakeRemoteImageFactory.NewRemoteReturnsOnCall(1, appImage, nil)

			newRunImage := fakes.NewImage("testbuilder/run@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0", "0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0", &fakeImageIdentifier{identifier: "runimage"})
			err = newRunImage.SetLabel("io.buildpacks.stack.id", "io.buildpacks.stacks.bionic")
			require.NoError(t, err)
			fakeRemoteImageFactory.NewRemoteReturnsOnCall(2, newRunImage, nil)

			rebasedAppImage := fakes.NewImage("testimage/app", "980723452toplayer", &fakeImageIdentifier{identifier: "rebasedappimage"})
			err = rebasedAppImage.SetLabel("io.buildpacks.build.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}]}`)
			require.NoError(t, err)
			err = rebasedAppImage.SetLabel("io.buildpacks.lifecycle.metadata", `{"runImage":{"topLayer":"sha256:719f3f610dade1fdf5b4b2473aea0c6b1317497cf20691ab6d184a9b2fa5c409","reference":"localhost:5000/node@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0"},"stack":{"runImage":{"image":"testbuilder/run:full-cnb"}}}`)
			require.NoError(t, err)
			fakeRemoteImageFactory.NewRemoteReturnsOnCall(3, rebasedAppImage, nil)

			imgRebaser = cnb.ImageRebaser{
				RemoteImageFactory: fakeRemoteImageFactory,
			}

			rebasedImage, err := imgRebaser.Rebase(build, context.TODO())
			require.NoError(t, err)

			assert.Equal(t, "index.docker.io/testbuilder/run@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0", rebasedImage.RunImage)
			imageName, secretRef := fakeRemoteImageFactory.NewRemoteArgsForCall(0)
			assert.Equal(t, "testbuilder/builder", imageName)
			assert.Equal(t, registry.SecretRef{
				ServiceAccount:   "testServiceAccount",
				Namespace:        "testNamespace",
				ImagePullSecrets: pullSecrets,
			}, secretRef)

			imageName, secretRef = fakeRemoteImageFactory.NewRemoteArgsForCall(1)
			assert.Equal(t, "testimage/app", imageName)
			assert.Equal(t, registry.SecretRef{
				ServiceAccount: "testServiceAccount",
				Namespace:      "testNamespace",
			}, secretRef)

		})
	})
}

type fakeImageIdentifier struct {
	identifier string
}

func (f *fakeImageIdentifier) String() string {
	return f.identifier
}
