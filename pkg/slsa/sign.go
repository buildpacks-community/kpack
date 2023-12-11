package slsa

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	ggcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
	slsav1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/cosign/v2/pkg/oci/mutate"
	cosignremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
)

const (
	DssePayloadType   = "application/vnd.dsse.envelope.v1+json"
	IntotoPayloadType = "application/vnd.in-toto+json"
)

func SignAndPush(ctx context.Context, digestStr string, stmt intoto.Statement, keychain authn.Keychain, signers ...dsse.Signer) (ggcrv1.Image, string, error) {
	payload, err := sign(ctx, stmt, signers...)
	if err != nil {
		return nil, "", err
	}

	opts := []static.Option{
		static.WithLayerMediaType(DssePayloadType),
		static.WithAnnotations(map[string]string{
			"predicateType": slsav1.PredicateSLSAProvenance,
		}),
	}

	attestation, err := static.NewAttestation(payload, opts...)
	if err != nil {
		return nil, "", err
	}

	ref, err := name.ParseReference(digestStr)
	if err != nil {
		return nil, "", err
	}

	attestationTag, err := cosignremote.AttestationTag(ref)
	if err != nil {
		return nil, "", err
	}

	img, err := cosignremote.Signatures(attestationTag)
	if err != nil {
		return nil, "", err
	}

	img, err = mutate.AppendSignatures(img, attestation)
	if err != nil {
		return nil, "", err
	}

	remoteOpts := []ggcrremote.Option{
		ggcrremote.WithContext(ctx),
	}
	if keychain != nil {
		remoteOpts = append(remoteOpts, ggcrremote.WithAuthFromKeychain(keychain))
	}

	err = ggcrremote.Write(attestationTag, img, remoteOpts...)
	if err != nil {
		return nil, "", err
	}

	signatureDigest, err := img.Digest()
	if err != nil {
		return nil, "", err
	}

	return img, fmt.Sprintf("%v@%v", attestationTag.Context().Name(), signatureDigest.String()), nil
}

func sign(ctx context.Context, stmt intoto.Statement, signers ...dsse.Signer) ([]byte, error) {
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

	return json.Marshal(envelope)
}
