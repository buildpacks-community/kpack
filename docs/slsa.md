# SLSA attestations

Kpack supports generating a [SLSA v1 provenance](https://slsa.dev/spec/v1.0/provenance) with each Build. These
attestations are written to the same registry as the app image and uses the same tag-based discovery mechanism as
[cosign](https://github.com/sigstore/cosign) for linking an image digest to an attestation image tag.

If enabled, an attestation will be generated for every newly completed [Build](./build.md) in the cluster. Kpack will
search through the secrets attached to the Build's service account, as well as the kpack-controller's service account
for signing keys. If at least one signing key is found, the attestation will be signed by all the keys. Otherwise an
unsigned attestation will be generated.

## Configuration

SLSA attestation can be enabled or disabled at the cluster level using the `EXPERIMENTAL_GENERATE_SLSA_ATTESTATION`
environment variable in the [kpack-controller's deployment](../config/controller.yaml).

## SLSA security level

Reference: https://slsa.dev/spec/v1.0/levels

By default, kpack provides `L0`, if SLSA attestation is enabled, it automatically achieves `L1`. For signed builds,
kpack achieves `L3` because:
- The build occurs on a Kubernetes cluster, usually this means it's on dedicated infrastructure but we won't judge you
  for running your cluster on kind. (L2)
- The signing private keys are provided via Kubernetes Secret, which can use RBAC to ensure minimal access. (L2)
- Builds are run in pods which are isolated from each other via Kubernetes principles. (L3)
- The only place the private keys are used to sign the attestation become accessible on the build pod is during the
  `completion` step, which is completely under the control of kpack. Even adding custom buildpacks to the Builder
  wouldn't allow access to the secrets. (L3)

## Provenance schema

Consult the documentation for the individual builder ID.

| Builder ID | Documentation |
|------------|---------------|
| `https://kpack.io/slsa/signed-app-build` | [slsa_build.md](./slsa_build.md) |
| `https://kpack.io/slsa/unsigned-app-build` | [slsa_build.md](./slsa_build.md) |

## Attestation storage

Attestations in kpack are attached to image digests and attests to the build environment of that particular image. As
such, the attestations are stored in a way that is predictable given an (app) image's digest. This is the same approach
that cosign uses and means the cosign CLI can be used to [verify kpack attestations](#verification-methods).

### Cosign tag-based discovery

Kpack attestations uses cosign's [tag-based
discovery](https://github.com/sigstore/cosign/blob/main/specs/SIGNATURE_SPEC.md#tag-based-discovery) with the only
difference that the suffix is `.att` instead of `.sig` (this also how `cosign attest` works). For an image digest
`registry.com/my/repo@sha256:1234`, the corresponding attestation will be uploaded to
`registry.com/my/repo:sha256-1234.att`.


### Storage format

The SLSA v1 _provenance_ is stored as a _predicate_ in an in-toto _statement_ which is base64 encoded and part of a DSSE
_envelope_. The envelope looks something like:

```json
{
  "payloadType": "application/vnd.in-toto+json",
  "payload": BASE64ENCODE({
      "_type": "https://in-toto.io/Statement/v0.1",
      "subject": [
        {
          "name": APP_IMAGE,
          "digest": {
            "sha256": APP_IMAGE_DIGEST
          }
        }
      "predicateType": "https://slsa.dev/provenance/v1",
      "predicate": SLSA V1 provenance...,
      ],
    })
  "signatures": [
    {
        "keyid": ...,
        "sig": ...,
    },
  ]
}
```

The envelope is stored as uncompressed text in the first layer of the attestation image. The image (and the registry)
is treated as a blobstore and isn't intended to be a container image. That is, `docker pull $ATTESTATION_TAG` or
trying to run the image in any way will **not** work.

If you want to access the attestation, you must use one of the tools that interact with the registry directly.

All of the following examples assume you have [jq](https://jqlang.github.io/jq/) installed. Given an `IMAGE_DIGEST`
`registry.com/my/repo@sha256:1234`, the `ATTESTATION_TAG` would be `registry.com/my/repo:sha256-1234.att`

The easiest way is to use [cosign](https://github.com/sigstore/cosign/blob/main/doc/cosign.md):
```bash
cosign download attestation $IMAGE_DIGEST | jq -r '.payload' | base64 --decode | jq
```

Another supported way is via [crane](https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md):
```bash
crane export $ATTESTATION_TAG | jq -r '.payload' | base64 --decode | jq
```

It's also accessible by [skopeo](https://github.com/containers/skopeo/blob/main/docs/skopeo.1.md), abeit with quite a
few more steps:
```bash
dir=$(mktemp -d)
skopeo copy docker://$ATTESTATION_TAG dir:$dir
sha=$(jq -r '.layers[0].digest | sub("^sha256:"; "")' $dir/manifest.json)
jq -r '.payload' $dir/$sha | base64 --decode | jq
rm -r $dir
```

## Signing keys

Build specific signing keys can be attached to the Service Account used for the Build. Cluster-wide signing keys can be
attached to the Service Account used in the `kpack-controller` Deployment in the system namespace (ususally `kpack`).

### PKCS#8 private key

A PKCS#8 private key using RSA, ECDSA, or ED25519 and stored in PEM format can be used to sign attestations. The private
key must use the same format as the [Kubernetes SSH auth secret](https://kubernetes.io/docs/concepts/configuration/secret/#ssh-authentication-secrets)
and have the `kpack.io/slsa: ""` annotation. Private keys with passwords are currently not supported.

``` yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/ssh-auth
metadata:
 name: my-ecdsa-key
 annotations:
   kpack.io/slsa: ""
stringData:
 ssh-privatekey: |
    -----BEGIN PRIVATE KEY-----
    <PRIVATE KEY DATA>
    -----END PRIVATE KEY-----
```

### Cosign private key

A  [cosign generated secret](https://github.com/sigstore/cosign/blob/main/doc/cosign_generate-key-pair.md) may also be
used as long as it has the `kpack.io/slsa: ""` annotation. Private keys with passwords are currently not supported.

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
 name: my-cosign-secret
 annotations:
   kpack.io/slsa: ""
data:
 cosign.key: <PRIVATE KEY DATA>
 cosign.password: <COSIGN PASSWORD>
 cosign.pub: <PUBLIC KEY DATA>
```

### Verification methods

A single signature consists of a `keyid` and a `sig` field where the `keyid` is the name of the Kubernetes Secret used
to generate the signature and the `sig` is the base64 encoded signature. The attestation will contain an array of these
signatures:

```json
{
    "payloadType": ...,
    "payload": ...,
    "signatures": [
      {
        "keyid": "cosign-secret",
        "sig": "MEQCID8QIkYOqxkPcE/bazsSDRj9vJSOXk9esFJSaj07jn2DAiB9/hrt8Ezd17UFYdaMSmMLzuF1oGSzK1vQ8jz5VSHNCQ=="
      },
      {
        "keyid": "rsa-secret",
        "sig": "s8NjZ7b7l0lGkJBeREJ9pP7kehXZWSY46413r06SIdVJbDxwgRlmF3HhK8Ji629yJs1jVLUgusBvexAM3ck+ZSzXOoOmT2sgLlvSNatF0F4iOJVA4/MFFYHOZokpObDZ/XDKC9DP8sI++x8gLhOvcPs7p/PtGXXnEJzOoedrHGV17Q1OOLIDPGkYP/CA+u0OANaAbipmaUUq7gY+E9JVKuSxHG91N9qzzvhl+dAIkbSruxMkhHkdA72OpYohKZ+Q0h+ChPI7XLrKJBKj5fBB4oOCE2a6+trKeBAwWAnlZDCN8wOWj602slQSCHpSqO9oi/u7X9aLCfhUsCZ5luY3iQ=="
      },
      {
        "keyid": "ecdsa-secret",
        "sig": "MEUCIQDEnkmqxb9ypLDIC+9oz7i5U22Tgq71YMVTf2tIuk+ubwIgZZfpAjLe8iW2Rp50PZz7DcUYvLGeG1NAMmGRlujy9S0="
      },
      {
        "keyid": "ed25519-secret",
        "sig": "WPGuhBYBlempQVC5BeULFeilJr3avQicH4MjruWsc8tUwL8dHgHxcONH6nNacRV9hKHO8wRJOSGs0Eot47aBDQ=="
      }
    ]
}
```

#### Cosign

To verify a cosign key, you can use the `cosign verify-attestation` command. This command will go through all the
signatures and verify at least one of them is signed by the public key. If you have access to the Kubernetes Namespace
(`$SECRET_NAMESPACE`) and Secret (`$SECRET_NAME`) containing the public-private keypair, you can use:

```bash
cosign verify-attestation --insecure-ignore-tlog=true --key k8s://$SECRET_NAMESPACE/$SECRET_NAME --type=slsaprovenance1 $APP_IMAGE_DIGEST
```

If you only have access to the file containing the public key (`$PUB_KEY_PATH`), you can use:

```bash
cosign verify-attestation --insecure-ignore-tlog=true --key $PUB_KEY_PATH --type=slsaprovenance1 $APP_IMAGE_DIGEST
```

#### PKCS#8

If you want to verify attestations signed by a PKCS#8 key (RSA, ECDSA, ED25519):

1. Grab and decode the base64 encoded payload from the attestation using one of the methods from [Storage format](#storage-format).
1. Compute the [DSSE PAE](https://github.com/secure-systems-lab/dsse/blob/v1.0.0/protocol.md) using `application/vnd.in-toto+json` as the type.
    This basically means filling in `DSSEv1 28 application/vnd.in-toto+json $NUM_BYTES_IN_PAYLOAD $PAYLOAD`
1. Grab and decode the base64 encoded signature you want to verify from the attestation.
1. Use `openssl` to verify the signature is correct for the PAE.

In practice this looks something like:

```bash
# Get attestation
ATTESTATION="$(cosign download attestation $APP_IMAGE_DIGEST)"
# Parse payload
PAYLOAD="$(echo $ATTESTATION | jq -r '.payload' | base64 --decode)"
# Parse signature, note: if you used multiple signing keys you will need to figure out which signature is from the key
# you want. Kpack does not provide any guranatees on the ordering used for signing.
echo $ATTESTATION | jq -r '.signatures[0].sig' | base64 --decode > message.sig
# Compute the PAE as message
echo -n $PAYLOAD | awk '{printf "DSSEv1 28 application/vnd.in-toto+json %d %s", length($0), $0}' > message.txt
```

To use a RSA or ECDSA key stored in PKCS#8 format, it must be verified against the SHA256 digest of the PAE:

```
openssl dgst -sha256 -binary message.txt | openssl pkeyutl -verify -pubin -inkey $PUB_KEY_PATH -pkeyopt digest:sha256 -sigfile message.sig
```

To use an ED25519 key stored in PKCS#8 public key, it can be verified directly against the PAE:

```
openssl pkeyutl -verify -pubin -inkey $PUB_KEY_PATH -sigfile message.sig -rawin -in message.txt
```
