# Problem
kpack does not provide users a way to validate the chain of custody for image tags that it builds. 
One very common way of accomplishing this is through the [Notary](https://github.com/theupdateframework/notary) project.

# Outcome
kpack integrates with Notary to sign images that it builds so that users are assured of the chain of custody.

# Actions to take

## Enable Image Singing
To use Notary image signing:
1. Add a new section to the image config that allows users to configure notary settings:
  ```
  notary:
    v1:
        url:       <notary server url>
        secretRef: <secret-name>
  ```
2. Add a new container after export that uses the notary go client to sign the image. The presence or absence of the notary section in the image config will determine if the notary signing container should be executed.
3.
    * The singing container would use `report.toml` to access the image digest. The image size would need to be added to `report.toml` or computed by the signer container.
      The `report.toml` must exist in its own volume as export does quite a bit of mangling to the layers volume.
    * Alternatively, the image digest and size could be computed by pulling the image in the signer container.

## Key Generation and Storage
kpack will not generate the targets public or private key pair used to sign images.
Instead, users will initialize a repo and generate target keys using the [Notary CLI](https://github.com/theupdateframework/notary/blob/master/docs/command_reference.md).
Users may accomplish this using the following notary commands:
```
notary init example.registry.io/my-app

notary key rotate example.registry.io/my-app snapshot -r

notary publish example.registry.io/my-app
```

Users must then upload the encrypted target private key and private key password as a generic k8s secret.
The secret data must be formatted as follows:
```
kubectl create secret generic notary-secret --from-file=notary-password.txt --from-file=~/.notary/private/<targets-hash>.key
```

That secret will be provided via the image config as described above.

## Authenticating with the Notary Server
If the Notary server requires authentication, users must create and attach a secret with basic auth Notary credentials to the service account used by the image/build config.
The secret must be properly annotated with the key `kpack.io/notary.v1` and the Notary server host name.
Given the secret with credentials, the authentication flow is described [here](https://github.com/theupdateframework/notary/blob/master/docs/service_architecture.md#example-client-server-signer-interaction) and [here](https://github.com/docker/distribution/blob/master/docs/spec/auth/token.md).

# Complexity
The actual integration with Notary is of low complexity.
Most of the necessary code is available in the open source docker CLI; particularly the files `cli/command/image/trust.go` and `cli/trust/trust.go`.
Additionally, the [notary-poc](https://github.com/pivotal/kpack/tree/notary-poc) branch has a proof-of-concept with example code.

# Prior Art
* The docker CLI implements this feature already.
* Some similarities to Tekton [Chains](https://github.com/tektoncd/chains).

# Alternatives
* Implement Notary signing directly in the lifecyle.
* Do not integrate with Notary.
* A solution that signs images outside of kpack entirely (aka. as another step in CI/CD).
* Use gpg keys much like Tekton [Chains](https://github.com/tektoncd/chains).

# Risks
* Notary is only one of the image singing tools available.
* Notary v2 is on the way.
* This scheme does not mention the notary CA certificate if there is one.
* This scheme does not handle delegation keys. Using delegation keys may simply be another option on the image configuration.
