# Images

Images provide a configuration for kpack to build and maintain a docker image utilizing [Cloud Native Buildpacks](http://buildpacks.io). 
kpack will monitor the inputs to the image configuration to rebuild the image when the underlying source or the builder's buildpacks or stacks have changed.       

The following defines the relevant fields of the `image` resource spec in more detail:

- `tag`: The image tag.
- `builder`: Configuration of the `builder` resource the image builds will use. See more info [Builder Configuration](builders.md).
- `serviceAccount`: The Service Account name that will be used for credential lookup.
- `source`: The source code that will be monitored/built into images. See the [Source Configuration](#source-config) section below.
- `cache`: Caching configuration, two variants are available:
  - `volume.size`: Creates a Volume Claim of the given size
  - `registry.tag`: Creates an image with cached contents
- `failedBuildHistoryLimit`: The maximum number of failed builds for an image that will be retained.
- `successBuildHistoryLimit`: The maximum number of successful builds for an image that will be retained.
- `imageTaggingStrategy`: Allow for builds to be additionally tagged with the build number. Valid options are `None` and `BuildNumber`.
- `build`: Configuration that is passed to every image build. See [Build Configuration](#build-config) section below.
- `projectDescriptorPath`: Path to the [project descriptor file](https://buildpacks.io/docs/reference/config/project-descriptor/) relative to source root dir or `subPath` if set. If unset, kpack will look for `project.toml` at the root dir or `subPath` if set.
- `notary`: Configuration for Notary image signing. See [Notary Configuration](#notary-config) section below.

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

> Note: This image can only reference builders defined in the same namespace. This is not true for ClusterBuilders because they are not namespace scoped.

### <a id='source-config'></a>Source Configuration

The `source` field is a composition of a source code location and a `subpath`. It can be configured in exactly one of the following ways:

* Git

    ```yaml
    source:
      git:
        url: ""
        revision: ""
      subPath: ""
    ```
    - `git`: (Source Code is a git repository)
        - `url`: The git repository url. Both https and ssh formats are supported; with ssh format requiring a [ssh secret](secrets.md#git-secrets).
        - `revision`: The git revision to use. This value may be a commit sha, branch name, or tag.
    - `subPath`: A subdirectory within the source folder where application code resides. Can be ignored if the source code resides at the `root` level.

* Blob

    ```yaml
    source:
      blob:
        url: ""
      subPath: ""
    ```
    - `blob`: (Source Code is a blob/jar in a blobstore)
        - `url`: The URL of the source code blob. This blob needs to either be publicly accessible or have the access token in the URL
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

The `build` field on the `image` resource can be used to configure env variables required during the build process, to configure resource limits on `CPU` and `memory`, and to configure pod tolerations, node selector, and affinity.

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

### <a id='notary-config'></a>Notary Configuration

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
- `<secret-name>`: The name of the secret. Ensure that the secret is created in the same namespace as the eventual image config.
- `<password>`: The password provided to encrypt the private key.
- `<hash>.key`: The private key file.

### Sample Image with a Git Source

```yaml
apiVersion: kpack.io/v1alpha1
kind: Image
metadata:
  name: sample-image
  namespace: build-namespace
spec:
  tag: gcr.io/project-name/app
  serviceAccount: service-account
  builder:
    name: sample-builder
    kind: ClusterBuilder
  cache:
    volume:
      size: "1.5Gi" # Optional, if not set then the caching feature is disabled
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

### Sample Image with hosted zip or jar as a source

```yaml
apiVersion: kpack.io/v1alpha1
kind: Image
metadata:
  name: sample-image
  namespace: build-namespace
spec:
  tag: gcr.io/project-name/app
  serviceAccount: service-account
  builder:
    name: sample-builder
    kind: ClusterBuilder
  cache:
    volume:
      size: "1.5Gi" # Optional, if not set then the caching feature is disabled
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

When an image has successfully built with its current configuration, its status will report the up to date fully qualified built image reference.

If you are using `kubectl` this information is available with `kubectl get <image-name>` or `kubectl describe <image-name>`. 

```yaml
status:
  conditions:
  - lastTransitionTime: "2020-01-17T16:16:36Z"
    status: "True"
    type: Succeeded
  latestImage: index.docker.io/sample/image@sha256:d3eb15a6fd25cb79039594294419de2328f14b443fa0546fa9e16f5214d61686
  ...
``` 

When a build fails its status will report the condition Succeeded=False. 

```yaml
status:
  conditions:
  - lastTransitionTime: "2020-01-17T16:13:48Z"
    status: "False"
    type: Succeeded
  ...
```
