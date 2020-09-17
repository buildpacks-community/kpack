# Problem
kpack does not handle image signing.

# Outcome
kpack integrates with notary to sign images that it builds.

# Actions to take
There are a few concrete steps to make this work in kpack:
1. Add a new section to the image config that allows users to configure notary settings:
  ```
  notary:
    host: <notary server url>
    rootPrivateKeyRef: <secret-name>
    rootPublicKeyRef: <secret-name> (may not be necessary)
    targetPrivateKeyRef: <secret-name>
    targetPublicKeyRef: <secret-name> (may not be necessary)
  ```
2. Add a new container after export that uses the notary go client to sign the image. The presence or absence of the notary section in the image config will determine if the notary signing container is executed.

There are a few different options to consider for the root and target keys:

**Key generation (2 Options)**
* kpack can auto generate the public/private key pair and create new secrets given the secret names in the image config. Auto generation will only happen on the first build of an image. Subsequent builds will use the keys from the secrets that were created. This is almost identical to how the docker CLI operates.
* kpack will require that the keys are generated and added to the cluster as secrets beforehand (aka. kpack does not auto generate the key pairs).

**Key storage (2 Options)**
* kpack stores both the public and private keys. The public key is only required for the first build of a new image.
* kpack stores only the private keys. This requires the users to independently upload the public keys to notary OR that kpack auto generate the key pair (the public key will only be persisted on the notary server).



# Complexity
The actual integration with notary is of low complexity. All of the necessary code is available in the open source docker CLI; particularly the files `cli/command/image/trust.go` and `cli/trust/trust.go`.

The complexity lies in the UX and the management of the root and target keys. Should kpack auto-generate public/private key pairs? Should kpack require users to interact with notary separate from kpack?

Finally, this scheme does not handle delegation keys. Using delegation keys may simply be another option on the image configuration.

# Prior Art
* The docker CLI implements this feature already.

# Alternatives
* Implement notary signing directly in the lifecyle.
* Do not integrate with notary.
* A solution that signs images outside of kpack entirely (aka. as another step in CI/CD).

# Risks
* Notary is only one of the image singing tools available.
* Notary v2 is on the way.
* Private notary keys are not really supposed to be stored on infrastructure.
