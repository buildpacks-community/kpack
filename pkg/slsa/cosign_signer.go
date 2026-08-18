package slsa

import (
	"bytes"
	"context"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/sigstore/pkg/signature"
)

var _ dsse.Signer = (*cosignSigner)(nil)

type cosignSigner struct {
	signer signature.Signer
	keyid  string
}

// NewCosignSigner loads a cosign private key into a dsse signer. The main difference between this signer and the one
// provided by sigstore's dsse.WrappedSigner is that this signer doesn't compute the PAE when signing
func NewCosignSigner(key, pass []byte, id string) (*cosignSigner, error) {
	sv, err := cosign.LoadPrivateKey(key, pass, cosign.GetDefaultLoadOptions(nil))
	if err != nil {
		return nil, err
	}

	return &cosignSigner{
		signer: sv,
		keyid:  id,
	}, nil
}

// KeyID implements dsse.Signer.
func (s *cosignSigner) KeyID() (string, error) {
	return s.keyid, nil
}

// Sign implements dsse.Signer.
func (s *cosignSigner) Sign(ctx context.Context, data []byte) ([]byte, error) {
	return s.signer.SignMessage(bytes.NewReader(data))
}
