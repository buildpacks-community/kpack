package cnb

import (
	"fmt"
	"io"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func layerFromStoreBuildpack(keychain authn.Keychain, buildpack buildapi.StoreBuildpack) (v1.Layer, error) {
	reference, err := name.ParseReference(buildpack.StoreImage.Image)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse %s", buildpack.StoreImage.Image)
	}

	fullyQualifiedLayer, err := name.NewDigest(fmt.Sprintf("%s@%s", reference.Context().Name(), buildpack.Digest))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to construct layer digest: %s", buildpack.BuildpackInfo)
	}

	return &remote.MountableLayer{
		Layer: &mountableBuildpackLayer{
			keychain:            keychain,
			fullyQualifiedLayer: fullyQualifiedLayer,
			digest:              buildpack.Digest,
			diffId:              buildpack.DiffId,
			size:                buildpack.Size,
		},
		Reference: reference,
	}, nil
}

type mountableBuildpackLayer struct {
	sync.Once
	keychain            authn.Keychain
	layer               v1.Layer
	fullyQualifiedLayer name.Digest
	digest              string
	diffId              string
	size                int64
}

func (m *mountableBuildpackLayer) Digest() (v1.Hash, error) {
	return v1.NewHash(m.digest)
}

func (m *mountableBuildpackLayer) DiffID() (v1.Hash, error) {
	return v1.NewHash(m.diffId)
}

func (m *mountableBuildpackLayer) Size() (int64, error) {
	return m.size, nil
}

func (m mountableBuildpackLayer) MediaType() (types.MediaType, error) {
	return types.DockerLayer, nil
}

func (m *mountableBuildpackLayer) Compressed() (io.ReadCloser, error) {
	err := m.fetchRemoteLayer()
	if err != nil {
		return nil, err
	}
	return m.layer.Compressed()
}

func (m *mountableBuildpackLayer) Uncompressed() (io.ReadCloser, error) {
	err := m.fetchRemoteLayer()
	if err != nil {
		return nil, err
	}
	return m.layer.Uncompressed()
}

func (m *mountableBuildpackLayer) fetchRemoteLayer() error {
	var err error
	m.Do(func() {
		m.layer, err = remote.Layer(m.fullyQualifiedLayer, remote.WithAuthFromKeychain(m.keychain))
	})
	return errors.Wrapf(err, "unable to construct remote layer")
}
