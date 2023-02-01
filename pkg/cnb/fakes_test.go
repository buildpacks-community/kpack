package cnb

import (
	"context"
	"fmt"
	"io"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

type fakeLayer struct {
	digest string
	diffID string
	size   int64
}

func (f fakeLayer) Digest() (v1.Hash, error) {
	return v1.NewHash(f.digest)
}

func (f fakeLayer) DiffID() (v1.Hash, error) {
	return v1.NewHash(f.diffID)
}

func (f fakeLayer) Size() (int64, error) {
	return f.size, nil
}

func (f fakeLayer) MediaType() (types.MediaType, error) {
	return types.DockerLayer, nil
}

func (f fakeLayer) Compressed() (io.ReadCloser, error) {
	panic("Not implemented For Tests")
}

func (f fakeLayer) Uncompressed() (io.ReadCloser, error) {
	panic("Not implemented For Tests")
}

type buildpackRefContainer struct {
	Ref       buildapi.BuilderBuildpackRef
	Buildpack K8sRemoteBuildpack
}

type fakeResolver struct {
	buildpacks         map[string]K8sRemoteBuildpack
	observedGeneration int64
}

func (r *fakeResolver) Resolve(ref buildapi.BuilderBuildpackRef) (K8sRemoteBuildpack, error) {
	buildpack, ok := r.buildpacks[fmt.Sprintf("%s@%s", ref.Id, ref.Version)]
	if !ok {
		return K8sRemoteBuildpack{}, errors.New("buildpack not found")
	}
	return buildpack, nil
}

func (f *fakeResolver) AddBuildpack(t *testing.T, ref buildapi.BuilderBuildpackRef, buildpack K8sRemoteBuildpack) {
	t.Helper()
	assert.NotEqual(t, ref.Id, "", "buildpack ref missing id")
	f.buildpacks[fmt.Sprintf("%s@%s", ref.Id, ref.Version)] = buildpack
}

func (r *fakeResolver) ClusterStoreObservedGeneration() int64 {
	return r.observedGeneration
}

func makeRef(id, version string) buildapi.BuilderBuildpackRef {
	return buildapi.BuilderBuildpackRef{
		BuildpackRef: corev1alpha1.BuildpackRef{
			BuildpackInfo: corev1alpha1.BuildpackInfo{
				Id:      id,
				Version: version,
			},
		},
	}
}

type fakeFetcher struct {
	buildpacks map[string][]buildpackLayer
}

func (f *fakeFetcher) Fetch(ctx context.Context, buildpack K8sRemoteBuildpack) (RemoteBuildpackInfo, error) {
	layers, ok := f.buildpacks[fmt.Sprintf("%s@%s", buildpack.Buildpack.Id, buildpack.Buildpack.Version)]
	if !ok {
		return RemoteBuildpackInfo{}, errors.New("buildpack not found")
	}

	return RemoteBuildpackInfo{
		BuildpackInfo: buildpackInfoInLayers(layers, buildpack.Buildpack.Id, buildpack.Buildpack.Version),
		Layers:        layers,
	}, nil
}

func (f *fakeFetcher) AddBuildpack(t *testing.T, id, version string, layers []buildpackLayer) {
	t.Helper()
	f.buildpacks[fmt.Sprintf("%s@%s", id, version)] = layers
}
