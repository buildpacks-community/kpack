package slsa

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
)

// NewPKCS8Signer can parse either a RSA, ECDSA, or ED25519 private key in PEM
// format and convert it into a dsse signer. It currently doesn't support
// encrypted keys.
//
// For RSA, this uses RSASSA-PKCS1-V1_5-SIGN with SHA256 as the hash function
// For ECDSA, this uses rand.Reader as the source for k
func NewPKCS8Signer(key []byte, id string) (Signer, error) {
	p, _ := pem.Decode([]byte(key))
	if p == nil {
		return nil, fmt.Errorf("failed to decode pem block for '%v'", id)
	}

	pkey, err := x509.ParsePKCS8PrivateKey(p.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %v", err)
	}

	switch pk := pkey.(type) {
	case *rsa.PrivateKey:
		return &rsaSigner{
			key:   pk,
			keyid: id,
		}, nil
	case *ecdsa.PrivateKey:
		return &ecdsaSigner{
			key:    pk,
			keyid:  id,
			randFn: rand.Reader,
		}, nil
	case ed25519.PrivateKey:
		return &ed25519Signer{
			key:   pk,
			keyid: id,
		}, nil
	default:
		return nil, fmt.Errorf("'%v' not a supported key type (rsa, ecdsa, ecdh)", id)
	}
}

var _ Signer = (*rsaSigner)(nil)
var _ Signer = (*ecdsaSigner)(nil)
var _ Signer = (*ed25519Signer)(nil)

type rsaSigner struct {
	key   *rsa.PrivateKey
	keyid string
}

func (s *rsaSigner) KeyID() (string, error) {
	return s.keyid, nil
}

func (s *rsaSigner) Sign(ctx context.Context, data []byte) ([]byte, error) {
	hf := crypto.SHA256
	hasher := hf.New()
	_, err := hasher.Write(data)
	if err != nil {
		return nil, fmt.Errorf("hashing data: %v", err)
	}

	return rsa.SignPKCS1v15(nil, s.key, crypto.SHA256, hasher.Sum(nil))
}

type ecdsaSigner struct {
	key    *ecdsa.PrivateKey
	keyid  string
	randFn io.Reader // this should be rand.Reader for anything other than tests
}

func (s *ecdsaSigner) KeyID() (string, error) {
	return s.keyid, nil
}

func (s *ecdsaSigner) Sign(ctx context.Context, data []byte) ([]byte, error) {
	return ecdsa.SignASN1(s.randFn, s.key, data)
}

type ed25519Signer struct {
	key   ed25519.PrivateKey
	keyid string
}

func (s *ed25519Signer) KeyID() (string, error) {
	return s.keyid, nil
}

func (s *ed25519Signer) Sign(ctx context.Context, data []byte) ([]byte, error) {
	return ed25519.Sign(s.key, data), nil
}
