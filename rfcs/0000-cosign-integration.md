# Integrate `cosign` with `kpack`

## Problem

The signing solution currently supported by `kpack` is
[Notary v1](https://github.com/theupdateframework/notary). However, Notary
presents itself with several different challenges, including:
- It is unable to sign container image digests. The user must always specify a
tag to be signed.
- It is unable to verify container image signature by digests.
- Signature relocation is not possible with this tool.
- It is a project that currently has low activity and community involvement,
  since the community is currently focused on the development of Notary v2.

[`cosign`](https://github.com/sigstore/cosign) presents itself as a solution to
some of those aforementioned challenges. With `cosign`, the user is able to:
- Relocate container images across registries along with its signatures, using
  either:
  - `crane cp` to copy images and signatures separately.
  - `cosign copy` to copy images along with their signatures from one registry
    to another.
- Sign container images by tag or by digest.
- Verify container image signatures by specifying either a tag or a digest.
- Verify container image signatures after relocation.

Moreover, `cosign` is a project that has had a lot of activity and community
involvement recently.

## Outcome

`kpack` integrates with `cosign` to sign images it builds and push the
signatures to a registry so that users can ensure the chain of custody of a
generated artifact.

This proposal aims to cover specifically the flow of signing an image produced
by `kpack Image` resource without verification of base or builder images pulled
in the process.

## Actions to take

### Enable image signing using `cosign`

- Implement a flow that generates the signature payload for the image, then
  calculates its signature and pushes it either to the registry where the image
  is located, using the same credentials that were used to push the image, or
  [to the registry specified in the `COSIGN_REPOSITORY` environment variable](#key-generation-and-storage).
  This flow must happen after the image has been pushed to the registry.

- If `cosign` fails to sign an image, the build should fail and output an error
  message in the build log, so the operator can troubleshoot the issue. The
  errors should also appear in any other places where error messages are
  presented.

- Whenever `kpack` signs an image using `cosign`, it should add these
  annotations:
  - Build number.
  - Build time.

- Signing with `cosign` should not affect any configurations that enable signing
  using the Notary v1 mechanism. These two signing mechanisms should be able to
  coexist.

### Key generation and storage

`cosign` can use file-based keys, key management systems or hardware keys to
ingest private keys, as well as a keyless flow that requires integration with
[Rekor](https://github.com/sigstore/rekor).

`kpack` should not generate the keys used for signing and verification. The user
should pass them in using one of the mechanisms supported by `cosign`.
In this RFC, we suggest that the first mechanism implemented be the use of
Kubernetes `Secret`s for sending the relevant data into the build pod, by means
of attaching secrets to the service account used by `kpack`.
These secrets should contain private key and private key passphrase.
Optionally, the user can add a repository URL to the secret, to address the use
case when pushing the signatures to a different registry is desired.

`kpack` will be able to determine that the user wants to sign images using
`cosign` by scanning the types of the secrets attached to the service account,
which in this use case would be a custom type:
```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: cosign-key
type: kpack.io/cosign-credentials
stringData:
  cosign-privatekey: <cosign-private-key>
  passphrase: <cosign-private-key-passphrase>
  # Optional repository URL, to be used as the environment variable
  # COSIGN_REPOSITORY. If not set, pushes the signature collocated with
  # the image.
  cosign-repository-url: <repository-url>
```

### Configure custom static annotations

Create a new optional configuration in the `Image` resource that allow users to
parameterize optional annotations to add to the generated signatures. These
annotations will be static and configured as a list of key-value pairs:
```yaml
cosign:
  annotations:
  - name: "key1"
    value: "value1"
  - name: "key2"
    value: "value2"
```

### Authentication to a container registry

`cosign` uses the `DefaultKeychain` from
[`go-containerregistry`](https://github.com/google/go-containerregistry/blob/main/pkg/authn/README.md#tldr-for-consumers-of-this-package)
to authenticate the command-line interface using the machine's Docker CLI
credentials. For `kpack`, it may be possible to use the same authentication
mechanism that is in-place for uploading an image.

## Complexity

Integration with `cosign` is estimated to be of medium complexity. Since
`cosign` is also written in Go, it is possible to import code from the tool
directly within `kpack`, which can help make the integration easier.

## Prior art

- [Issue](https://github.com/pivotal/kpack/issues/684) filed on May 4th.
- [Use of `cosign` in Kubernetes](https://github.com/kubernetes/release/pull/2016)
  for validation of distroless images.
- [Use of `cosign` in Connaisseur](https://github.com/sse-secure-systems/connaisseur/pull/107)
  for policy enforcement in container images.
- [Notary v1 implementation pull request](https://github.com/pivotal/kpack/pull/541).

## Alternatives

- Use a CI/CD step to sign images using `cosign` instead of implementing it as
  a part of `kpack`.

## Risks

- Notary v2 support might be a requirement in the future.
- `cosign` is at an early stage of development and may have breaking changes in
  the future.
