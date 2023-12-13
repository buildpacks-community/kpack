package slsa

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
	slsav1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	cosignremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	cosignstatic "github.com/sigstore/cosign/v2/pkg/oci/static"
)

type Signer = dsse.Signer

const (
	DssePayloadType   = "application/vnd.dsse.envelope.v1+json"
	IntotoPayloadType = "application/vnd.in-toto+json"
)

func (*Attester) Sign(ctx context.Context, stmt intoto.Statement, signers ...Signer) ([]byte, error) {
	payload, err := json.Marshal(stmt)
	if err != nil {
		return nil, fmt.Errorf("marshalling statement: %v", err)
	}
	pae := dsse.PAE(IntotoPayloadType, payload)

	sigs := make([]dsse.Signature, len(signers))
	for i, signer := range signers {
		keyId, err := signer.KeyID()
		if err != nil {
			return nil, fmt.Errorf("retrieving keyid: %v", err)
		}

		sig, err := signer.Sign(ctx, pae)
		if err != nil {
			return nil, fmt.Errorf("signing payload using '%v': %v", keyId, err)
		}

		sigs[i] = dsse.Signature{
			KeyID: keyId,
			Sig:   base64.StdEncoding.EncodeToString(sig),
		}
	}

	envelope := dsse.Envelope{
		PayloadType: IntotoPayloadType,
		Payload:     base64.StdEncoding.EncodeToString(payload),
		Signatures:  sigs,
	}

	b, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshalling envelope: %v", err)
	}

	return b, nil
}

func (*Attester) Write(ctx context.Context, digestStr string, payload []byte, keychain authn.Keychain) (ggcrv1.Image, string, error) {
	opts := []cosignstatic.Option{
		cosignstatic.WithLayerMediaType(DssePayloadType),
		cosignstatic.WithAnnotations(map[string]string{
			"predicateType": slsav1.PredicateSLSAProvenance,
		}),
	}

	attestation, err := cosignstatic.NewAttestation(payload, opts...)
	if err != nil {
		return nil, "", fmt.Errorf(":%v", err)
	}

	ref, err := name.ParseReference(digestStr)
	if err != nil {
		return nil, "", fmt.Errorf(":%v", err)
	}

	attestationTag, err := cosignremote.AttestationTag(ref)
	if err != nil {
		return nil, "", fmt.Errorf(":%v", err)
	}

	annots, err := attestation.Annotations()
	if err != nil {
		return nil, "", fmt.Errorf("getting attestation annotations: %v", err)
	}

	// Overwrite any existing attestations with a new one. The only time this is
	// relevant is when multiple builds result in bit-for-bit same images (since
	// the digest would be the same in both builds).
	img := scratchImage()
	img, err = mutate.Append(img, mutate.Addendum{
		Layer:       attestation,
		Annotations: annots,
	})
	if err != nil {
		return nil, "", err
	}

	remoteOpts := []remote.Option{
		remote.WithContext(ctx),
	}
	if keychain != nil {
		remoteOpts = append(remoteOpts, remote.WithAuthFromKeychain(keychain))
	}

	err = remote.Write(attestationTag, img, remoteOpts...)
	if err != nil {
		return nil, "", fmt.Errorf(":%v", err)
	}

	signatureDigest, err := img.Digest()
	if err != nil {
		return nil, "", fmt.Errorf(":%v", err)
	}

	return img, fmt.Sprintf("%v@%v", attestationTag.Context().Name(), signatureDigest.String()), nil
}

// TODO: figure out how to determine if we should use the default docker media
// type. since all the secrets/signatures are combined into a single
// attestation image, it'll probably have to be on the Build resource
func scratchImage() ggcrv1.Image {
	img := empty.Image
	img = mutate.MediaType(img, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, types.OCIConfigJSON)
	return img
}
