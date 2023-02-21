# Buildpacks

The Buildpack, ClusterBuildpack, and ClusterStore resources are a repository of
[buildpacks](http://buildpacks.io/) packaged in
[buildpackages](https://buildpacks.io/docs/buildpack-author-guide/package-a-buildpack/)
that can be used by kpack to build OCI images.

These resources are to be used with the [Builder and
ClusterBuilder](builders.md) resources.

### <a id='cnb-buildpacks'></a>Difference from CNB buildpacks

Kpack will allow any image with a valid `io.buildpacks.buildpack.layers` label
to be used as the source of a buildpackage. This means that you can also use a
CNB builder image (i.e. `index.docker.io/paketobuildpacks/builder-jammy-base`)
in addition to regular CNB buildpack images (i.e.
`gcr.io/paketo-buildpacks/java:8.9.0`).

### <a id='buildpack'></a>Buildpack Configuration

Buildpack is a namespaced resource that represents an external buildpackage.

```yaml
apiVersion: kpack.io/v1alpha2
kind: Buildpack
metadata:
  name: sample-buildpack
spec:
  serviceAccountName: sample-sa
  source:
    # image: gcr.io/paketo-buildpacks/java
    # image: gcr.io/paketo-buildpacks/java:8.9.0
    image: gcr.io/paketo-buildpacks/java@sha256:fc1c6fba46b582f63b13490b89e50e93c95ce08142a8737f4a6b70c826c995de
```

* `serviceAccountName`: A service account with the sercrets needed to pull the
  buildpack image.
* `source.image`: The uri of the buildpackage.

### <a id='cluster-buildpack'></a>Cluster Buildpack Configuration

ClusterBuildpack is the cluster scoped version of Buildpack

```yaml
apiVersion: kpack.io/v1alpha2
kind: ClusterBuildpack
metadata:
  name: sample-cluster-buildpack
spec:
  serviceAccountRef:
    name: sample-sa
    namespace: sample-namespace
  source:
    # image: gcr.io/paketo-buildpacks/java
    # image: gcr.io/paketo-buildpacks/java:8.9.0
    image: gcr.io/paketo-buildpacks/java@sha256:fc1c6fba46b582f63b13490b89e50e93c95ce08142a8737f4a6b70c826c995de
```

* `serviceAccountRef`: An object reference to a service account in any
  namespace. The object reference must contain `name` and `namespace`.
* `source.image`: The uri of the buildpackage.


### <a id='cluster-store'></a>Cluster Store Configuration

ClusterStore is a cluster scoped resource that can reference multiple buildpackages.

Corresponding `kp` cli command docs
[here](https://github.com/vmware-tanzu/kpack-cli/blob/main/docs/kp_clusterstore.md).

```yaml
apiVersion: kpack.io/v1alpha2
kind: ClusterStore
metadata:
  name: sample-cluster-store
spec:
 serviceAccountRef:
    name: sample-sa
    namespace: sample-namespace
  sources:
  - image: gcr.io/cf-build-service-public/node-engine-buildpackage@sha256:95ff756f0ef0e026440a8523f4bab02fd8b45dc1a8a3a7ba063cefdba5cb9493
  - image: gcr.io/cf-build-service-public/npm-buildpackage@sha256:5058ceb9a562ec647ea5a41008b0d11e32a56e13e8c9ec20c4db63d220373e33
  - image: index.docker.io/paketobuildpacks/builder-jammy-base
```

* `serviceAccountRef`: An object reference to a service account in any
  namespace. The object reference must contain `name` and `namespace`.
* `sources`:  List of buildpackage images to make available in the
  ClusterStore. Each image is an object with the key image.


### Updating Buildpacks

The Buildpack, ClusterBuildpack, and ClusterStore resources will not poll for
updates. A CI/CD tool is needed to update the resource with new digests when
new images are available.

### Suggested buildpackages

The most commonly used buildpackages are [paketo buildpacks](https://paketo.io/).

### Creating your own buildpackage

To create your own buildpackage with custom buildpacks follow the instructions
on creating them and packaging them using the [pack
cli](https://buildpacks.io/docs/buildpack-author-guide/).
