# Problem
kpack does not handle image signing.

# Outcome
kpack integrates with notary to sign images that it builds.

# Actions to take
There are a few concrete steps to make this work in kpack:
1. Add a new section to the image config that allows users to configure notary settings:
  ```
  notary:
    host:                <notary server url>
    rootPrivateKeyRef:   <secret-name>
    targetPrivateKeyRef: <secret-name>
  ```
2. Add a new container after export that uses the notary go client to sign the image. The presence or absence of the notary section in the image config will determine if the notary signing container is executed.

## Key Generation and Storage
kpack will auto generate the public/private key pair and create new secrets given the secret names in the image config. Auto generation will only happen on the first build of an image. Subsequent builds will use the keys from the secrets that were created. Additionally, kpack will not store the public key, letting the notary server store them instead.

# Complexity
The actual integration with notary is of low complexity. All of the necessary code is available in the open source docker CLI; particularly the files `cli/command/image/trust.go` and `cli/trust/trust.go`.

# Prior Art
* The docker CLI implements this feature already.

# Alternatives
* Implement notary signing directly in the lifecyle.
* Do not integrate with notary.
* A solution that signs images outside of kpack entirely (aka. as another step in CI/CD).
* kpack could let users provide the public/private key pair instead of generating them.
* kpack could store both the private key and the public key.

# Risks
* Notary is only one of the image singing tools available.
* Notary v2 is on the way.
* Private notary keys are not really supposed to be stored on infrastructure.
* This scheme does not handle the notary CA certificate if there is one.
* This scheme does not handle delegation keys. Using delegation keys may simply be another option on the image configuration.
