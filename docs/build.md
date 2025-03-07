# Builds

A Build is a resource that schedules and run a single [Cloud Native Buildpacks](http://buildpacks.io) build.

Corresponding `kp` cli command docs [here](https://github.com/vmware-tanzu/kpack-cli/blob/main/docs/kp_build.md).

Unlike with the [Image resource](image.md), using Builds directly allows granular control of when builds execute. Each build resource is immutable and corresponds to a single build execution. You will need to create a new build for every build execution as builds will not rebuild on source code and buildpack updates. Additionally, you will need to manually specify the source, and the cache volume. 

A Build resource is comparable to `pack build`. Are you familiar with pack? if so, you can check the [comparison section](kpack-vs-pack.md) 

### Configuration

```yaml
apiVersion: kpack.io/v1alpha2
kind: Build
metadata:
  name: sample-build
spec:
  tags:
  - sample/image
  serviceAccountName: service-account
  builder:
    image: gcr.io/paketo-buildpacks/builder:base
    imagePullSecrets:
    - name: builder-secret
  cache:
    volume:
      persistentVolumeClaimName: persisent-volume-claim-name
  projectDescriptorPath: path/to/project.toml
  source:
    git:
      url: https://github.com/buildpack/sample-java-app.git
      revision: main
  activeDeadlineSeconds: 1800
  env:
  - name: "JAVA_BP_ENV"
    value: "value"
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

- `tags`: A list of docker tags to build. At least one tag is required.
- `serviceAccount`: The Service Account name that will be used for credential lookup. Check out the [secrets documentation](secrets.md) for more information. 
- `builder.image`: This is the tag to the [Cloud Native Buildpacks builder image](https://buildpacks.io/docs/using-pack/working-with-builders/) to use in the build. Unlike on the Image resource, this is an image not a reference to a Builder resource.    
- `builder.imagePullSecrets`: An optional list of pull secrets if the builder is in a private registry. [To create this secret please reference this link](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#registry-secret-existing-credentials)
- `source`: The source location that will be the input to the build. See the [Source Configuration](#source-config) section below.
- `activeDeadlineSeconds`: Optional configurable max active time that the pod can run for
- `cache`: Caching configuration, two variants are available:
  - `volume.persistentVolumeClaimName`: Optional name of a persistent volume claim used for a build cache across builds.
  - `registry.tag`: Optional name of a tag used for a build cache across builds.
- `env`: Optional list of build time environment variables.
- `defaultProcess`: The [default process type](https://buildpacks.io/docs/app-developer-guide/run-an-app/) for the built OCI image
- `projectDescriptorPath`: Path to the [project descriptor file](https://buildpacks.io/docs/reference/config/project-descriptor/) relative to source root dir or `subPath` if set. If unset, kpack will look for `project.toml` at the root dir or `subPath` if set.
- `resources`: Optional configurable resource limits on `CPU` and `memory`.
- `tolerations`: Optional configurable pod spec tolerations
- `nodeSelector`: Optional configurable pod spec nodeSelector
- `affinity`: Optional configurabl pod spec affinity

> Note: All fields on a build are immutable. Instead of updating a build, create a new one.
 
##### <a id='source-config'></a>Source Configuration

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
        - `auth`: Optional auth to use with blob source. Leave empty for no auth, "secret" for providing auth [via Secret](secrets.md#blob-secrets), or "helper" to use service account IAM (specific to each IaaS).
             > Note: Only [Microsoft Azure](https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview)
             > and [Google Cloud Platform](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#kubernetes-sa-to-iam)
             > helpers are currently implemented, contributions are welcome to `pkg/blob/<iaas>_keychain.go`.
    - `subPath`: A subdirectory within the source folder where application code resides. Can be ignored if the source code resides at the `root` level.

* Blob

    ```yaml
    source:
      blob:
        url: ""
        stripComponents: 0
      subPath: ""
    ```
    - `blob`: (Source Code is a blob/jar in a blobstore)
        - `url`: The URL of the source code blob. This blob needs to either be publicly accessible or have the access token in the URL
        - `stripComponents`: Optional number of directory components to strip from the blobs content when extracting.
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



#### Status

When a build complete successfully its status will report the fully qualified built image reference.

If you are using `kubectl` this information is available with `kubectl get <build-name>` or `kubectl describe <build-name>`. 

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
