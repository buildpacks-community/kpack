package cnb

import (
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
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
