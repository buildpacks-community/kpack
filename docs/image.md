# Image Resources

Image resources provide a configuration for kpack to build and maintain a docker image utilizing [Cloud Native Buildpacks](http://buildpacks.io).
kpack will monitor the inputs to the image resource to rebuild the OCI image when the underlying source or the builder's buildpacks or stacks have changed.

Corresponding `kp` cli command docs [here](https://github.com/vmware-tanzu/kpack-cli/blob/main/docs/kp_image.md).

The following defines the relevant fields of the `image` resource spec in more detail:

- `tag`: The image tag.
- `additionalTags`: Any additional list of image tags that should be published. This list of tags is mutable.
- `builder`: Configuration of the `builder` resource the image builds will use. See more info [Builder Configuration](builders.md).
- `serviceAccountName`: The Service Account name that will be used for credential lookup.
- `source`: The source code that will be monitored/built into images. See the [Source Configuration](#source-config) section below.
- `cache`: Caching configuration, two variants are available:
  - `volume.size`: Creates a Volume Claim of the given size
  - `volume.storageClassName`: (Optional) Creates a Volume Claim of the given storageClassName. If unset, the default storage class is used. The field is immutable.
  - `registry.tag`: Creates an image with cached contents
- `failedBuildHistoryLimit`: The maximum number of failed builds for an image that will be retained.
- `successBuildHistoryLimit`: The maximum number of successful builds for an image that will be retained.
- `imageTaggingStrategy`: Allow for builds to be additionally tagged with the build number. Valid options are `None` and `BuildNumber`.
- `build`: Configuration that is passed to every image build. See [Build Configuration](#build-config) section below.
- `defaultProcess`: The [default process type](https://buildpacks.io/docs/app-developer-guide/run-an-app/) for the built OCI image
- `projectDescriptorPath`: Path to the [project descriptor file](https://buildpacks.io/docs/reference/config/project-descriptor/) relative to source root dir or `subPath` if set. If unset, kpack will look for `project.toml` at the root dir or `subPath` if set.
- `cosign`: Configuration for additional cosign image signing. See [Cosign Configuration](#cosign-config) section below.

### <a id='tags-config'></a> Configuring Tags

The `tag` field is the location the built OCI image will be written to for each build. This field is immutable.

Examples:

- `tag: dockerhubuser/repo`
- `tag: dockerhubuser/repo:my-image`
- `tag: gcr.io/project/repo`

The `additionalTags` is a list of locations the built OCI image will be written to in addition to the `tag`. Additional tags must be in the same registry as the `tag`. Cross registry exporting is not supported. This field can be modified.

Example:

```yaml
tag: my-registry.io/project/repo 
additionalTags:
- my-registry.io/project/repo:some-version
- my-registry.io/project/repo:some-metadata
- my-registry.io/project/other-repo
```

### <a id='builder-config'></a>Builder Configuration

The `builder` field describes the [builder resource](builders.md) that will build the OCI images for a provided image configuration. It can be defined in exactly one of the following ways:

* Cluster Builder

    ```yaml
    builder:
        name: cluster-builder-name
        kind: ClusterBuilder
    ```
    - `name`: The name of the ClusterBuilder resource in kubernetes.
    - `kind`: The type as defined in kubernetes. This will always be ClusterBuilder.

* Namespaced Builder

    ```yaml
    builder:
        name: builder-name
        kind: Builder
    ```
    - `name`: The name of the Builder resource in kubernetes.
    - `kind`: The type as defined in kubernetes. This will always be Builder.

> Note: This image resource can only reference builders defined in the same namespace. This is not true for ClusterBuilders because they are not namespace scoped.

### <a id='source-config'></a>Source Configuration

The `source` field is a composition of a source code location and a `subpath`. It can be configured in exactly one of the following ways:

* Git

    ```yaml
    source:
      git:
        url: ""
        revision: ""
        initializeSubmodules: false
      subPath: ""
    ```
    - `git`: (Source Code is a git repository)
        - `url`: The git repository url. Both https and ssh formats are supported; with ssh format requiring a [ssh secret](secrets.md#git-secrets).
        - `revision`: The git revision to use. This value may be a commit sha, branch name, or tag.
        - `initializeSubmodules`: Initialize submodules inside repo, recurses up to a max depth of 10 submodules.
    - `subPath`: A subdirectory within the source folder where application code resides. Can be ignored if the source code resides at the `root` level.

* Blob

    ```yaml
    source:
      blob:
        url: ""
        stripComponents: 0
        auth: "" | "secret" | "helper"
      subPath: ""
    ```
    - `blob`: (Source Code is a blob/jar in a blobstore)
        - `url`: The URL of the source code blob. This blob needs to either be publicly accessible or have the access token in the URL
        - `stripComponents`: Optional number of directory components to strip from the blobs content when extracting.
        - `auth`: Optional auth to use with blob source. Leave empty for no auth, "secret" for providing auth [via Secret](secrets.md#blob-secrets), or "helper" to use service account IAM (specific to each IaaS).
             > Note: Only [Microsoft Azure](https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview)
             > and [Google Cloud Platform](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#kubernetes-sa-to-iam)
             > helpers are currently implemented, contributions are welcome to `pkg/blob/<iaas>_keychain.go`.
    - `subPath`: A subdirectory within the source folder where application code resides. Can be ignored if the source code resides at the `root` level.

* Registry

    ```yaml
    source:
      registry:
        image: ""
        imagePullSecrets:
        - name: ""
      subPath: ""
    ```
    - `registry` ( Source code is an OCI image in a registry that contains application source)
        - `image`: Location of the source image
        - `imagePullSecrets`: A list of `dockercfg` or `dockerconfigjson` secret names required if the source image is private
    - `subPath`: A subdirectory within the source folder where application code resides. Can be ignored if the source code resides at the `root` level.

### <a id='build-config'></a>Build Configuration

The `build` field on the `image` resource can be used to configure env variables required during the build process, to configure resource limits on `CPU` and `memory`, and to configure pod tolerations, node selector, build timeout (specified in seconds), and affinity. To configure "Creation Time" of the built app image, pass in the unix EPOCH timestamp (i.e "1667243396") as a string or use "now" to use the current time.  

```yaml
build:
  env:
    - name: "name of env variable"
      value: "value of the env variable"
  resources:
      requests:
        cpu: "0.25"
        memory: "128M"
      limits:
        cpu: "0.5"
        memory: "256M"
  tolerations:
    - key: "key1"
      operator: "Exists"
      effect: "NoSchedule"
  nodeSelector:
    disktype: ssd
  buildTimeout: 1600
  creationTime: "now"
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: kubernetes.io/e2e-az-name
                operator: In
                values:
                  - e2e-az1
                  - e2e-az2
```

See the kubernetes documentation on [setting environment variables](https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-container/) and [resource limits and requests](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container) for more information.

### <a id='cosign-config'></a>Cosign Configuration

#### Cosign Signing Secret
OCI images built by kpack can be signed with cosign when a cosign formatted secret is added to the service account configured on the image resource.
The secret can be added using the cosign CLI or manually. 

To create a cosign signing secret through the cosign CLI, when targetted to the Kubernetes cluster, use:
`cosign generate-key-pair k8s://[NAMESPACE]/[NAME]`

Alternatively, create the cosign secret and provide your own cosign key files manually to Kubernetes by running the following command:
```shell script
% kubectl create secret generic <secret-name> --from-literal=cosign.password=<password> --from-file=</path/to/cosign.key>
```
- `<secret-name>`: The name of the secret. Ensure that the secret is created in the same namespace as the eventual image resource.
- `<password>`: The password provided to encrypt the private key. If not present, an empty password will be used.
- `</path/to/cosign.key>`: The cosign private key file generated with `cosign generate-key-pair`.

After adding the cosign secret, the secret must be added to the list of `secrets` on the service account that the image is configured with.

#### Adding Cosign Annotations
By default, the build number and build timestamp information will be added to the cosign signing annotations. Users can specify additional cosign annotations under the spec key.
```yaml
cosign:
  annotations:
  - name: "annotationName"
    value: "annotationValue"
```

One way these annotations can be viewed is through verifying cosign signatures. The annotations will be under the `optional` key in the verified JSON response. For example, this can be done with:
```bash
% cosign verify -key /path/to/cosign.pub registry.example.com/project/image@sha256:<DIGEST>
```

Which provides a JSON response similar to:
```json
{
  "critical": {
    "identity": {
      "docker-reference": "registry.example.com/project/image"
    }, "image": {
      "docker-manifest-digest": "sha256:<DIGEST>"
    }, "type": "cosign container image signature"
  }, "optional": {
    "buildNumber": "1",
    "buildTimestamp": "20210827.175240",
    "annotationName": "annotationValue"
  }
}
```

#### Push Cosign Signature to a Different Location
Cosign signatures can be pushed to a different registry from where the built OCI image is written to. To enable this, add the corresponding annotation to the cosign secret resource.
```
metadata:
  name: ...
  namespace: ...
  annotations:
    kpack.io/cosign.repository: other.registry.com/project/image
data:
  cosign.key: ...
  cosign.password: ...
```
This will be equivalent to setting `COSIGN_REPOSITORY` as specified in cosign [Specifying Registry](https://github.com/sigstore/cosign#specifying-registry)

The service account configured on the image resource must have registry credentials for the other registry configured in the `secrets` list (they do not need to be configured as `imagePullSecrets`).

It should be noted that if you wish to push the signatures to the same registry but a different repository as the image resource `tag`, the credential used must have access to both paths. You cannot use two separate credentials for the same registry.

#### Cosign Legacy Docker Media Types
To sign docker images in a registry that does not fully support OCI media types, legacy equivalents can be used by adding the corresponding annotation to the cosign secret resource:
```
metadata:
  name: ...
  namespace: ...
  annotations:
    kpack.io/cosign.docker-media-types: "1"
data:
  cosign.key: ...
  cosign.password: ...
```
This will be equivalent to setting `COSIGN_DOCKER_MEDIA_TYPES=1` as specified in the cosign [registry-support](https://github.com/sigstore/cosign#registry-support)

### Sample Image Resource with a Git Source

```yaml
apiVersion: kpack.io/v1alpha2
kind: Image
metadata:
  name: sample-image
  namespace: build-namespace
spec:
  tag: gcr.io/project-name/app
  serviceAccountName: service-account
  builder:
    name: sample-builder
    kind: ClusterBuilder
  cache:
    volume:
      size: "1.5Gi" # Optional, if not set then the caching feature is disabled
      storageClassName: "my-storage-class" # Optional, if not set then the default storageclass is used
  failedBuildHistoryLimit: 5 # Optional, if not present defaults to 10
  successBuildHistoryLimit: 5 # Optional, if not present defaults to 10
  source:
    git:
      url: https://github.com/buildpack/sample-java-app.git
      revision: main
  build: # Optional
    env:
      - name: BP_JAVA_VERSION
        value: 8.*
    resources:
      limits:
        cpu: 100m
        memory: 1G
      requests:
        cpu: 50m
        memory: 512M
```

Source for github can also be specified in the ssh format if there is a corresponding ssh secret

```yaml
  source:
    git:
      url: git@github.com/buildpack/sample-java-app.git
      revision: main
```

### Sample Image Resource with hosted zip or jar as a source

```yaml
apiVersion: kpack.io/v1alpha2
kind: Image
metadata:
  name: sample-image
  namespace: build-namespace
spec:
  tag: gcr.io/project-name/app
  serviceAccountName: service-account
  builder:
    name: sample-builder
    kind: ClusterBuilder
  cache:
    volume:
      size: "1.5Gi" # Optional, if not set then the caching feature is disabled
      storageClassName: "my-storage-class" # Optional, if not set then the default storageclass is used
  failedBuildHistoryLimit: 5 # Optional, if not present defaults to 10
  successBuildHistoryLimit: 5 # Optional, if not present defaults to 10
  source:
    blob:
      url: https://storage.googleapis.com/build-service/sample-apps/spring-petclinic-2.1.0.BUILD-SNAPSHOT.jar
  build: # Optional
    env:
      - name: BP_JAVA_VERSION
        value: 8.*
    resources:
      limits:
        cpu: 100m
        memory: 1G
      requests:
        cpu: 50m
        memory: 512M
```

#### Status

When an image resource has successfully built with its current configuration, its status will report the up to date fully qualified built OCI image reference.

If you are using `kubectl` this information is available with `kubectl get <image-resource-name>` or `kubectl describe <image-resource-name>`.

```yaml
status:
  conditions:
  - lastTransitionTime: "2020-01-17T16:16:36Z"
    status: "True"
    type: Ready
  latestImage: index.docker.io/sample/image@sha256:d3eb15a6fd25cb79039594294419de2328f14b443fa0546fa9e16f5214d61686
  ...
``` 

When a build fails its status will report the condition Succeeded=False. 

```yaml
status:
  conditions:
  - lastTransitionTime: "2020-01-17T16:13:48Z"
    status: "False"
    type: Ready
  ...
```

### Legacy apiVersion kpack.io/v1alpha1

Notable deprecations from `kpack.io/v1alpha1` include:
- Notary image signing
- [Cloud Native Buildpack service bindings](docs/legacy-cnb-servicebindings.md)

`kpack.io/v1alpha1` will eventually be removed entirely so please migrate existing Image resources to use `kpack.io/v1alpha2` apis.

#### <a id='notary-config'></a>Notary Configuration

`apiVersion` must be `kpack.io/v1alpha1`

The optional `notary` field on the `image` resource can be used to configure [Notary](https://github.com/theupdateframework/notary) image signing.
```yaml
notary:
  v1:
    url: "https://example.com/notary"
    secretRef:
      name: "notary-secret"
```
- `v1.url`: The URL of the notary server.
- `v1.secretRef.name`: A [secret](#notary-secret) containing the encrypted private key and private key password.

#### Generate Signing Key
To generate a signing key, use the following commands from the [Docker Content Trust](https://docs.docker.com/engine/security/trust/#signing-images-with-docker-content-trust) documentation:
```shell script
% export DOCKER_CONTENT_TRUST_SERVER=<notary-server-url>
% docker trust key generate my-key
% docker trust signer add --key my-key.pub my-key registry.example.com/org/app
```
This will generate a private key in `~/.docker/trust/private` encrypted with the user provided password.

#### <a id='notary-secret'></a>Create Notary Secret
To create the notary secret used by kpack for image signing, run the following command:
```shell script
% kubectl create secret generic <secret-name> --from-literal=password=<password> --from-file=$HOME/.docker/trust/private/<hash>.key
```
- `<secret-name>`: The name of the secret. Ensure that the secret is created in the same namespace as the eventual image resource.
- `<password>`: The password provided to encrypt the private key.
- `<hash>.key`: The private key file.
