# Stores

A store resource is a repository of [buildpacks](http://buildpacks.io/) packaged in [buildpackages](https://buildpacks.io/docs/buildpack-author-guide/package-a-buildpack/) that can be used by kpack to build OCI images.

The store will be referenced by a [builder](builders.md) resource.

At this time only a Cluster scoped `ClusterStore` is available.

Corresponding `kp` cli command docs [here](https://github.com/vmware-tanzu/kpack-cli/blob/main/docs/kp_clusterstore.md).

### <a id='cluster-store'></a>Cluster Store Configuration

```yaml
apiVersion: kpack.io/v1alpha2
kind: ClusterStore
metadata:
  name: sample-cluster-store
spec:
  sources:
  - image: gcr.io/cf-build-service-public/node-engine-buildpackage@sha256:95ff756f0ef0e026440a8523f4bab02fd8b45dc1a8a3a7ba063cefdba5cb9493
  - image: gcr.io/cf-build-service-public/npm-buildpackage@sha256:5058ceb9a562ec647ea5a41008b0d11e32a56e13e8c9ec20c4db63d220373e33
  - image: gcr.io/paketo-buildpacks/builder:base
```

* `sources`:  List of buildpackage images to make available in the ClusterStore. Each image is an object with the key image.

> Note: ClusterBuilders will also work with a prebuilt builder image if a builpack is not available in a buildpackage.

### Using a private registry

To use the buildpackage images from a private registry, you have to add a `serviceAccountRef` referencing a serviceaccount with the secrets needed to pull from this registry.

```yaml
spec:
 serviceAccountRef:
    name: private
    namespace: private
```

* `serviceAccountRef`: An object reference to a service account in any namespace. The object reference must contain `name` and `namespace`.

### Updating a store

The store resource will not poll for updates. A CI/CD tool is needed to update the resource with new digests when new images are available.

### Suggested buildpackages

The most commonly used buildpackages are [paketo buildpacks](https://paketo.io/).

### Creating your own buildpackage

To create your own buildpackage with custom buildpacks follow the instructions on creating them and packaging them using the [pack cli](https://buildpacks.io/docs/buildpack-author-guide/).

