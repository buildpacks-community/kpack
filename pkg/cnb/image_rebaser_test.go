package cnb

import (
	"context"
	"fmt"
	"testing"

	"github.com/buildpack/imgutil"
	"github.com/buildpack/imgutil/fakes"
	"github.com/google/go-cmp/cmp"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry"
)

func TestImageRebase(t *testing.T) {
	spec.Run(t, "ImageRebaser", testImageRebaser)
}

func testImageRebaser(t *testing.T, when spec.G, it spec.S) {
	const (
		namespace           = "testNamespace"
		builder             = "testbuilder/builder@sha256:afb5ef696a93eda86fd491a81fd21c3b4a423e9ce8807ea3e908516a6336e479"
		buildServiceAccount = "testServiceAccount"
	)
	var (
		imgRebaser             ImageRebaser
		fakeRemoteImageFactory = &FakeRemoteImageUtilFactory{}
		builderPullSecrets     = []corev1.LocalObjectReference{
			{
				Name: "some-secret",
			},
		}
	)

	when("#Rebase", func() {
		it("invokes Rebase on the previously built image", func() {
			build := &v1alpha1.Build{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testBuild",
					Namespace: namespace,
				},
				Spec: v1alpha1.BuildSpec{
					Tags: []string{"testimage/app", "additional/tags"},
					Builder: v1alpha1.BuildBuilderSpec{
						Image:            builder,
						ImagePullSecrets: builderPullSecrets,
					},
					ServiceAccount: buildServiceAccount,
					LastBuild:      v1alpha1.LastBuild{Image: "testimage/app@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0"},
				},
			}

			builderImage := fakes.NewImage("testbuilder/builder", "293847toplayer", &fakeImageIdentifier{identifier: "builder"})
			require.NoError(t, builderImage.SetLabel("io.buildpacks.builder.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}], "stack": { "runImage": { "image": "foo.io/run:basecnb" }}}`))

			appImage := fakes.NewImage("testimage/app", "980723452toplayer", &fakeImageIdentifier{identifier: "testimage/app@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0"})
			require.NoError(t, appImage.SetLabel("io.buildpacks.lifecycle.metadata", `{"runImage":{"topLayer":"sha256:719f3f610dade1fdf5b4b2473aea0c6b1317497cf20691ab6d184a9b2fa5c409","reference":"localhost:5000/node@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0"},"stack":{"runImage":{"image":"foo.io/run:basecnb"}}}`))
			require.NoError(t, appImage.SetLabel("io.buildpacks.build.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}]}`))
			require.NoError(t, appImage.SetLabel("io.buildpacks.stack.id", "io.buildpacks.stacks.bionic"))

			newRunImage := fakes.NewImage("foo.io/run:basecnb", "0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0", &fakeImageIdentifier{identifier: "foo.io/run@sha256:c4e5e3ea177cd1238f67481d920ea17388792a0fb2cfa38fd95394f912c35ea8"})
			require.NoError(t, newRunImage.SetLabel("io.buildpacks.stack.id", "io.buildpacks.stacks.bionic"))

			fakeRemoteImageFactory.NewRemoteReturnsForArgs(newRemoteArgs{
				ImageName: builder,
				BaseImage: builder,
				SecretRef: registry.SecretRef{
					Namespace:        namespace,
					ImagePullSecrets: builderPullSecrets,
				},
			}, builderImage)
			fakeRemoteImageFactory.NewRemoteReturnsForArgs(
				newRemoteArgs{
					ImageName: "testimage/app",
					BaseImage: "testimage/app@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0",
					SecretRef: registry.SecretRef{
						Namespace:      namespace,
						ServiceAccount: buildServiceAccount,
					},
				}, appImage)
			fakeRemoteImageFactory.NewRemoteReturnsForArgs(newRemoteArgs{
				ImageName: "foo.io/run:basecnb",
				BaseImage: "foo.io/run:basecnb",
				SecretRef: registry.SecretRef{
					Namespace:        namespace,
					ImagePullSecrets: builderPullSecrets,
				},
			}, newRunImage)

			imgRebaser = ImageRebaser{
				RemoteImageFactory: fakeRemoteImageFactory,
			}

			rebasedImage, err := imgRebaser.Rebase(build, context.TODO())
			require.NoError(t, err)

			assert.Equal(t, "foo.io/run@sha256:c4e5e3ea177cd1238f67481d920ea17388792a0fb2cfa38fd95394f912c35ea8", rebasedImage.RunImage)

			assert.Len(t, appImage.SavedNames(), 2)
			assert.Contains(t, appImage.SavedNames(), "testimage/app")
			assert.Contains(t, appImage.SavedNames(), "additional/tags")
		})
	})
}

type fakeImageIdentifier struct {
	identifier string
}

func (f *fakeImageIdentifier) String() string {
	return f.identifier
}

type FakeRemoteImageUtilFactory struct {
	args   []newRemoteArgs
	images []imgutil.Image
}

type newRemoteArgs struct {
	ImageName string
	BaseImage string
	SecretRef registry.SecretRef
}

func (f *FakeRemoteImageUtilFactory) newRemote(imageName string, baseImage string, secretRef registry.SecretRef) (imgutil.Image, error) {
	for i, args := range f.args {
		remoteArgs := newRemoteArgs{ImageName: imageName, BaseImage: baseImage, SecretRef: secretRef}
		if diff := cmp.Diff(args, remoteArgs); diff == "" {
			return f.images[i], nil
		}
	}

	return nil, fmt.Errorf("image %s not configured", imageName)

}

func (f *FakeRemoteImageUtilFactory) NewRemoteReturnsForArgs(args newRemoteArgs, image *fakes.Image) {
	f.args = append(f.args, args)
	f.images = append(f.images, image)
}
