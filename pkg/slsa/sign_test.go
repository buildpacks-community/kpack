package slsa

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
	slsacommon "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
	slsav1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/sclevine/spec"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/attest"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/verify"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/stretchr/testify/require"
)

func TestSigner(t *testing.T) {
	spec.Run(t, "Test signer", testSigner)
}

func testSigner(t *testing.T, when spec.G, it spec.S) {
	var (
		statement intoto.Statement
		attester  = Attester{}

		ctx       = context.Background()
		timestamp = time.Date(2023, time.January, 1, 1, 0, 0, 0, time.UTC)
	)

	it.Before(func() {
		statement = intoto.Statement{
			StatementHeader: intoto.StatementHeader{
				Type:          intoto.StatementInTotoV01,
				PredicateType: slsav1.PredicateSLSAProvenance,
				Subject: []intoto.Subject{
					{
						Name: "subject",
						Digest: slsacommon.DigestSet{
							"sha256": "some-sha",
						},
					},
				},
			},
			Predicate: slsav1.ProvenancePredicate{
				BuildDefinition: slsav1.ProvenanceBuildDefinition{
					BuildType: "build-type",
					ExternalParameters: map[string]interface{}{
						"external": "param",
					},
					InternalParameters: map[string]interface{}{
						"internal": "param",
					},
					ResolvedDependencies: []slsav1.ResourceDescriptor{},
				},
				RunDetails: slsav1.ProvenanceRunDetails{
					Builder: slsav1.Builder{
						ID: "unsigned",
						Version: map[string]string{
							"some": "version",
						},
						BuilderDependencies: []slsav1.ResourceDescriptor{},
					},
					BuildMetadata: slsav1.BuildMetadata{
						InvocationID: "some-invocation-id",
						StartedOn:    &timestamp,
						FinishedOn:   &timestamp,
					},
					Byproducts: []slsav1.ResourceDescriptor{},
				},
			},
		}
	})

	when("signing statements", func() {
		formatPayload := func(sigs string) string {
			return fmt.Sprintf(`{"payloadType":"application/vnd.in-toto+json","payload":"eyJfdHlwZSI6Imh0dHBzOi8vaW4tdG90by5pby9TdGF0ZW1lbnQvdjAuMSIsInByZWRpY2F0ZVR5cGUiOiJodHRwczovL3Nsc2EuZGV2L3Byb3ZlbmFuY2UvdjEiLCJzdWJqZWN0IjpbeyJuYW1lIjoic3ViamVjdCIsImRpZ2VzdCI6eyJzaGEyNTYiOiJzb21lLXNoYSJ9fV0sInByZWRpY2F0ZSI6eyJidWlsZERlZmluaXRpb24iOnsiYnVpbGRUeXBlIjoiYnVpbGQtdHlwZSIsImV4dGVybmFsUGFyYW1ldGVycyI6eyJleHRlcm5hbCI6InBhcmFtIn0sImludGVybmFsUGFyYW1ldGVycyI6eyJpbnRlcm5hbCI6InBhcmFtIn19LCJydW5EZXRhaWxzIjp7ImJ1aWxkZXIiOnsiaWQiOiJ1bnNpZ25lZCIsInZlcnNpb24iOnsic29tZSI6InZlcnNpb24ifX0sIm1ldGFkYXRhIjp7Imludm9jYXRpb25JRCI6InNvbWUtaW52b2NhdGlvbi1pZCIsInN0YXJ0ZWRPbiI6IjIwMjMtMDEtMDFUMDE6MDA6MDBaIiwiZmluaXNoZWRPbiI6IjIwMjMtMDEtMDFUMDE6MDA6MDBaIn19fX0=","signatures":[%v]}`, sigs)
		}

		it("outputs the correct format when no signer is present", func() {
			bytes, err := attester.Sign(ctx, statement)
			require.NoError(t, err)

			expected := formatPayload("")
			require.Equal(t, expected, string(bytes))
		})

		it("outputs the correct format when rsa signer is used", func() {
			p, _ := pem.Decode([]byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXQIBAAKBgQC6NYrd7Fk+VVZ6/5bhA32kKybmEhnP87GnNXHmJFRZRiG1uy4d
hyFFPP/RzaBtX5p2v1CyYCCdsbGAFC+AwYY0WPeUxoH9CoHfbVNARzP2hEpxf1UE
wbElSXEV5g+ORWEhM5kCiepawUOuJDRBlkJv+NsgNJLO3W4iAWpGBSWKQwIDAQAB
AoGBAJEkKODAV/OROv5WeLldKjKre/96/mmr7r6Blw0nJiQL2Ty48MINUS6R1Gkp
WTHHgjOq/fB7oxCnr85AbaJztfGEsoOBRQwpuhyxs9atwkzfqfdvP41gYxE5ZOSR
SVA2PZZ3YpayTFGr0aue0Z/0uDNr1r0r2Yzi4FEHsYRaMn3pAkEA52JPPaTn6Beo
RwVBmISsz0foH8oERXA+TiuO+7ntUI3I+0Qu0gHCOG4nUMosg+rjSrf1s9TrPTy6
wqphnL081QJBAM4E59yVB97IswDqpBKDUHPhwt+gL+tHr0l1rV9Sigf8P+JzJ+qW
3T2mjA9zShsql1HAHVhKa+9jUBUGyLNx1rcCQEZg8rVPWXKQ75Bamu8nO+D4gDRb
IB123JyuRxluHi7oUX+L56nc+xRq/O5n8E5XvbbWaZe/ZaYEuNHJPVONg9kCQQCw
NxH7Yp2wuQojJo/gJeGSvO/qRyG1pUMRGk6I0hiua9xY3RavWQszyFjUflBMnEmB
60kuU7musMu2sG7KzfxRAkB4IQT4WeUZQewWYLC3wOefwGhBaqqG0aQvTAxNgCKw
S5CYclcJQ7nokUAdf5OjKmAPa5dDnYnVcebi0Bk31GLA
-----END RSA PRIVATE KEY-----`))
			k, err := x509.ParsePKCS1PrivateKey(p.Bytes)
			require.NoError(t, err)

			signer := &rsaSigner{
				key:   k,
				keyid: "some-rsa-key",
			}

			bytes, err := attester.Sign(ctx, statement, signer)
			require.NoError(t, err)

			// Note: the golang stdlib RSA PKCS1v15 signing is deterministic, so we get to enjoy
			// hardcoding the signature. Other libraries and online checkers aren't neccessarily so.
			expected := formatPayload(`{"keyid":"some-rsa-key","sig":"qyhZenrI9bhGs3MUFSNTJXtJvYqN5ZuJF3TVlhl7U/zPjGHk1s5DOsZoIgp0V8sxGpah96k5YIxAMDcW96JgAWfe1FT3FhNTH43ZPI4nkbk/FCZRQHhIxkOhp6rUQY4C+rI3e/EWLCUO6bWouH0+VzN1BbtXLTlqm3+/eM1BPRI="}`)
			require.Equal(t, expected, string(bytes))
		})

		it("outputs the correct format when ecdsa signer is used", func() {
			p, _ := pem.Decode([]byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIML570JqmxT3O3QJYI6/yhM+EklNoUZBOCQwWzxJx/VeoAoGCCqGSM49
AwEHoUQDQgAEct985zOnUCYL7rW84V0M3N78XHL4vZok3YBvjpWb6p1gIVim9CET
P4amRng1j+1PnrdDixxQJtmAZT1lJZdXvQ==
-----END EC PRIVATE KEY-----`))
			k, err := x509.ParseECPrivateKey(p.Bytes)
			require.NoError(t, err)

			signer := &ecdsaSigner{
				key:   k,
				keyid: "some-ecdsa-key",
				// ecdsa utilizes a random k during the signing process which normally makes it
				// nondeterministic. so we force a static prng to make it work for our tests
				randFn: &constReader{c: byte(0)}}

			bytes, err := attester.Sign(ctx, statement, signer)
			require.NoError(t, err)

			expected := formatPayload(`{"keyid":"some-ecdsa-key","sig":"MEUCIQCBPU7x0+KOBggqbfGGLFjsRilVD94yzOMPnDuAWhAVfgIgPDA5A2NwqPTo2ABJ/KNahJy38QfljO9GtuafK0RI5F0="}`)
			require.Equal(t, expected, string(bytes))
		})

		it("outputs the correct format when ed25519 signer is used", func() {
			p, _ := pem.Decode([]byte(`-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIATRP4Od4Mta/KjTO7c99nfGL/PCUn9Grn7mnXCiIXuW
-----END PRIVATE KEY-----`))
			k, err := x509.ParsePKCS8PrivateKey(p.Bytes)
			require.NoError(t, err)

			signer := &ed25519Signer{
				key:   k.(ed25519.PrivateKey),
				keyid: "some-ed25519-key",
			}

			bytes, err := attester.Sign(ctx, statement, signer)
			require.NoError(t, err)

			expected := formatPayload(`{"keyid":"some-ed25519-key","sig":"f4Ch73gK9ZBrM1uD+ifTffZ2sQfiQcBRQpUOBa0TCFN5/nIGnce7VXxB8t8fL1aD7OGCIxeovSKsrbt54dNZCA=="}`)
			require.Equal(t, expected, string(bytes))
		})

		when("parsing pkcs#8 keys", func() {
			it("parses rsa key", func() {
				signer, err := NewPKCS8Signer([]byte(`-----BEGIN PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBAMuMYS7JFwpUhTQv
/qgY1cE9VKaWCHxE7GXhAjp6CdZHdSvld/p6YhNFh3q3ra8xyNJ8TLzzgckilBWN
E4TL9C7X+0OuXJgqPs9DDrWU1leKxNkHW/VLocdaBGcMO3ObExBMSQmqxx7aZXDE
ZgsXpTxN6dFZi0NXdZQFxHWbmAIFAgMBAAECgYEAwloXsQRw63woc9wf24FbGFfn
K+i10XI+BkkCR7OfEOrKBk/kb1XLr3sEETiwrkdq6Q+mdgRMODC8FJfPu0zCqQEl
N09GzgWdiLUqFCjI5mDH45pknVUqErev3UjOQXtHc/mJShtzYYXOZIv/TEtQ5qlx
nJyw5UwWdotl3Q5LM0ECQQD+tw1HU3087IJiAyEtdT+P6EjdyN83xEvUN4lsL0pX
oSSGND86uGdegEhLeIqSUaArNcHxRLzOEhBU6fFZutR3AkEAzJM/z1C5vSnrqk/x
iVz5LQId/0GPuJaxH88BZPVn3cExFonP2ekoOk7krreY9US+QvCEYoaIpMRQTMkS
FZToYwJAddxT654pRCiYCCw0jOlE+RzdoqpRB2fwQolUMFgNiVDKKsNAQraTNjob
ubhUPnJAr93XGQHab2wSXIhHomJ97wJAGZ1+aekDuMnc9lBYQAjiMl6HfwSoiEZm
G0OCOy/3N28PRHExE2n8XYCvZakbf+qBoEQZb3TE+0yReWvb1dP1XQJANi9sUQYB
KOV35SGKCGhuMKW3pwyPNc418C1a8cBStWNufVg6MjgCgFOcWYEByuUXEIveonPu
ReiVnVr2kT44fA==
-----END PRIVATE KEY-----`), "some-rsa-key")
				require.NoError(t, err)

				bytes, err := attester.Sign(ctx, statement, signer)
				require.NoError(t, err)

				expected := formatPayload(`{"keyid":"some-rsa-key","sig":"FqnF6A48fXhdW3xuTcRe1rChnyosp/Dap0y2Y+u4qe/Xd3zXuCR0mbejw8cxk5jqbewDwby/Uw8g+cINYR8H733AM8x1tRYOCuHMdAphATL+7KcESn9eNuKnOT2ioOsdrKHNqmpbHXJ8JcRLLhVLlVcJ7nX5RLja9+2PNqYIkAU="}`)
				require.Equal(t, expected, string(bytes))
			})

			// ECDSA isn't tested because signing with a random k isn't
			// deterministic, and it's individually tested above

			it("parses ed25519 keys", func() {
				signer, err := NewPKCS8Signer([]byte(`-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIATRP4Od4Mta/KjTO7c99nfGL/PCUn9Grn7mnXCiIXuW
-----END PRIVATE KEY-----`), "some-ed25519-key")
				require.NoError(t, err)

				bytes, err := attester.Sign(ctx, statement, signer)
				require.NoError(t, err)

				expected := formatPayload(`{"keyid":"some-ed25519-key","sig":"f4Ch73gK9ZBrM1uD+ifTffZ2sQfiQcBRQpUOBa0TCFN5/nIGnce7VXxB8t8fL1aD7OGCIxeovSKsrbt54dNZCA=="}`)
				require.Equal(t, expected, string(bytes))
			})

			it("errors on bogus keys", func() {
				signer, err := NewPKCS8Signer([]byte(`some-bogus-key`), "some-bogus-key")
				require.Nil(t, signer)
				require.Error(t, err)
			})
		})
	})

	when("compared with cosign cli", func() {
		var (
			server     *httptest.Server
			sinkLogger = log.New(io.Discard, "", 0)

			repo, digest, attTag    string
			privKeyFile, pubKeyFile string
			predicateFile           string
		)

		it.Before(func() {
			server = httptest.NewServer(registry.New(registry.Logger(sinkLogger)))

			repo = fmt.Sprintf("%v/some-image", strings.TrimPrefix(server.URL, "http://"))
			digest = fmt.Sprintf("%v@sha256:2074f12a86b413824be18bf6471e7b6b9c13bce83832fe18efc635591d9cb1d3", repo)
			attTag = fmt.Sprintf("%v:sha256-2074f12a86b413824be18bf6471e7b6b9c13bce83832fe18efc635591d9cb1d3.att", repo)

			statement.Subject = []intoto.Subject{{
				Name: repo,
				Digest: slsacommon.DigestSet{
					"sha256": "2074f12a86b413824be18bf6471e7b6b9c13bce83832fe18efc635591d9cb1d3",
				},
			}}

			privKeyFile, pubKeyFile = generateCosignKey(t)

			f, err := os.CreateTemp("", "")
			require.NoError(t, err)
			defer f.Close()

			b, err := json.Marshal(statement.Predicate)
			require.NoError(t, err)

			_, err = f.Write(b)
			require.NoError(t, err)
			predicateFile = f.Name()
		})

		it("generates the same image tag", func() {
			// attest image via cosign
			cmd := attest.AttestCommand{
				KeyOpts:        options.KeyOpts{KeyRef: privKeyFile},
				TlogUpload:     false,
				PredicateType:  options.PredicateSLSA1,
				PredicatePath:  predicateFile,
				RekorEntryType: "dsse",
			}
			err := cmd.Exec(ctx, digest)
			require.NoError(t, err)

			ref1, err := name.ParseReference(attTag)
			require.NoError(t, err)
			img1, err := remote.Image(ref1)
			require.NoError(t, err)

			// delete image so we can reuse the same digest w/o appending signatures
			err = remote.Delete(ref1)
			require.NoError(t, err)

			// attest image via our implementation
			signer := loadCosignSigner(t, privKeyFile)
			payload, err := attester.Sign(ctx, statement, signer)
			require.NoError(t, err)
			img2, _, err := attester.Write(ctx, digest, payload, nil)
			require.NoError(t, err)

			// assert attestation images are the same
			// note that because cryptographic signings aren't deterministic (a random k is generated each
			// time), we can't assert on digest or contents
			size1, err := img1.Size()
			require.NoError(t, err)
			size2, err := img2.Size()
			require.NoError(t, err)

			require.Equal(t, size1, size2)
		})

		it("is verifiable by cosign", func() {
			// sign image via our implementation
			signer := loadCosignSigner(t, privKeyFile)
			payload, err := attester.Sign(ctx, statement, signer)
			require.NoError(t, err)
			_, _, err = attester.Write(ctx, digest, payload, nil)
			require.NoError(t, err)

			// attest image via cosign
			cmd := verify.VerifyAttestationCommand{
				IgnoreTlog:    true,
				KeyRef:        pubKeyFile,
				PredicateType: options.PredicateSLSA1,
			}
			err = cmd.Exec(ctx, []string{digest})
			require.NoError(t, err, "Result differs from `cosign verify-attestation`")
		})
	})
}

func generateCosignKey(t *testing.T) (string, string) {
	t.Helper()

	keys, err := cosign.GenerateKeyPair(nil)
	require.NoError(t, err)

	privKey, err := os.CreateTemp("", "")
	require.NoError(t, err)
	defer privKey.Close()

	err = privKey.Chmod(0600)
	require.NoError(t, err)
	_, err = privKey.Write(keys.PrivateBytes)
	require.NoError(t, err)

	pubKey, err := os.CreateTemp("", "")
	require.NoError(t, err)
	defer pubKey.Close()

	err = pubKey.Chmod(0644)
	require.NoError(t, err)
	_, err = pubKey.Write(keys.PublicBytes)
	require.NoError(t, err)

	return privKey.Name(), pubKey.Name()
}

func loadCosignSigner(t *testing.T, keyFile string) Signer {
	t.Helper()
	b, err := os.ReadFile(keyFile)
	require.NoError(t, err)

	s, err := NewCosignSigner(b, nil, "")
	require.NoError(t, err)
	return s
}

type constReader struct {
	c byte
}

func (r *constReader) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = r.c
	}
	return len(b), nil
}
