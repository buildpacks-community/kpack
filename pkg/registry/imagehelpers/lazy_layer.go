package imagehelpers

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
)

type LazyMountableLayerArgs struct {
	Digest, DiffId, Image string
	Size                  int64
	Keychain              authn.Keychain
}

func NewLazyMountableLayer(args LazyMountableLayerArgs) (v1.Layer, error) {
	reference, err := name.ParseReference(args.Image)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse %s", args.Image)
	}

	fullyQualifiedLayer, err := name.NewDigest(fmt.Sprintf("%s@%s", reference.Context().Name(), args.Digest))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to construct layer digest: %s", args.Digest)
	}

	return &remote.MountableLayer{
		Layer: &lazyMountableLayer{
			keychain:            args.Keychain,
			fullyQualifiedLayer: fullyQualifiedLayer,
			digest:              args.Digest,
			diffId:              args.DiffId,
			size:                args.Size,
		},
		Reference: reference,
	}, nil
}

type lazyMountableLayer struct {
	sync.Once
	keychain            authn.Keychain
	layer               v1.Layer
	fullyQualifiedLayer name.Digest
	digest              string
	diffId              string
	size                int64
}

func (m *lazyMountableLayer) Digest() (v1.Hash, error) {
	return v1.NewHash(m.digest)
}

func (m *lazyMountableLayer) DiffID() (v1.Hash, error) {
	return v1.NewHash(m.diffId)
}

func (m *lazyMountableLayer) Size() (int64, error) {
	return m.size, nil
}

func (m lazyMountableLayer) MediaType() (types.MediaType, error) {
	return types.DockerLayer, nil
}

func (m *lazyMountableLayer) Compressed() (io.ReadCloser, error) {
	err := m.fetchRemoteLayer()
	if err != nil {
		return nil, err
	}
	return m.layer.Compressed()
}

func (m *lazyMountableLayer) Uncompressed() (io.ReadCloser, error) {
	err := m.fetchRemoteLayer()
	if err != nil {
		return nil, err
	}
	return m.layer.Uncompressed()
}

func (m *lazyMountableLayer) fetchRemoteLayer() error {
	var err error
	m.Do(func() {
		m.layer, err = remote.Layer(m.fullyQualifiedLayer, remote.WithAuthFromKeychain(m.keychain))
	})
	return errors.Wrapf(err, "unable to construct remote layer")
}
