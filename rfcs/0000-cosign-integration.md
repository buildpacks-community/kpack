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
- Relocate container images across registries along with its signatures.
  - Currently, there is no built-in command for relocation on `cosign`, but
    copying container images and signatures across registries using other
    container CLI tools such as `crane` and then verifying with `cosign` is
    possible.
  - There is also a proposal for
    [implementing a `cosign cp`](https://github.com/sigstore/cosign/issues/303)
    command that would affect this copy as a single operation.
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

- Create a new configuration in the `Image` resource that allow users to
  configure image signing with `cosign` as the signing mechanism, specifying
  the secret reference to get the private key and passphrase. The user can also
  specify signature annotations as part of configuration:
```yaml
cosign:
    secretRef: <secret-name> # private key and passphrase to sign images
    annotations: # optional map of key-value pairs the user wants to add to images
      key1: value1
      key2: value2
```
`cosign` will apply this optional map of annotations to the signature payload,
which can then be verified with `cosign verify`. Verification is out of scope
for this RFC.

- Implement a flow that generates the signature payload for the image, then
  calculates its signature and pushes it to the registry where the image is
  located, using the same credentials that were used to push the image. This
  flow must happen after the image has been pushed to the registry.

- If `cosign` fails to sign an image, the build should fail and output an error
  message specifying the failure.

- Whenever `kpack` signs an image using `cosign`, it should add these
  annotations:
  - Build number.
  - Cluster builder used.
  - Buildpacks involved in the build process.
  - Stack container image names and digests.
  - Build time.

### Key generation and storage

`cosign` can use file-based keys, key management systems or hardware keys to
ingest private keys, as well as a keyless flow that requires integration with
[Rekor](https://github.com/sigstore/rekor).

`kpack` should not generate the keys used for signing and verification. The user
should pass it in using one of the mechanisms supported by `cosign`. In this RFC,
we suggest that the first mechanism implemented be the use of Kubernetes
`Secret`s for sending private key and passphrase into the build pod:
```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: cosign-key
type: Opaque
data:
  cosign-privatekey: # base64 encoded private key here
  passphrase: # base64 encoded private key passphrase here
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
