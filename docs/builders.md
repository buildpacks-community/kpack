# Builders

kpack provides Builder and ClusterBuilder resources to define and create [Cloud Native Buildpacks builders](https://buildpacks.io/docs/using-pack/working-with-builders/) all within the kpack api.
This allows granular control of how stacks, buildpacks, and buildpack versions are utilized and updated.

Before creating Builders you will need to create a [ClusterStack](stack.md) and [ClusterStore](store.md) resources below.

> Note: The Builder and ClusterBuilder were previously named CustomBuilder and CustomClusterBuilder. The previous Builder and ClusterBuilder resources that utilized pre-built builders were removed and should no longer be used with kpack. This was discussed in an approved [RFC](https://github.com/pivotal/kpack/pull/439).

Corresponding `kp` cli command docs for [builders](https://github.com/vmware-tanzu/kpack-cli/blob/main/docs/kp_builder.md) and [cluster builders](https://github.com/vmware-tanzu/kpack-cli/blob/main/docs/kp_clusterbuilder.md).

### <a id='builders'></a>Builders

The Builder uses a [ClusterStore](store.md), a [ClusterStack](stack.md), and an order definition to construct a builder image.

```yaml
apiVersion: kpack.io/v1alpha2
kind: Builder
metadata:
  name: my-builder
spec:
  tag: gcr.io/sample/builder
  serviceAccountName: default
  stack:
    name: bionic-stack
    kind: ClusterStack
  store:
    name: sample-cluster-store
    kind: ClusterStore
  order:
  - group:
    - id: paketo-buildpacks/java
  - group:
    - id: paketo-buildpacks/nodejs
  - group:
    - id: kpack/my-custom-buildpack
      version: 1.2.3
    - id: kpack/my-optional-custom-buildpack
      optional: true
```

* `tag`: The tag to save the builder image. You must have access via the referenced service account.
* `serviceAccount`: A service account with credentials to write to the builder tag.
* `order`: The [builder order](https://buildpacks.io/docs/reference/builder-config/). See the [Order](#order) section below.
* `stack.name`: The name of the stack resource to use as the builder stack. All buildpacks in the order must be compatible with the clusterStack.
* `stack.kind`: The type as defined in kubernetes. This will always be ClusterStack.
* `store.name`: The name of the ClusterStore resource in kubernetes.
* `store.kind`: The type as defined in kubernetes. This will always be ClusterStore.

### <a id='cluster-builders'></a>Cluster Builders

The ClusterBuilder resource is almost identical to a Builder but, it is a cluster scoped resource that can be referenced by an image in any namespace. Because ClusterBuilders are not in a namespace they cannot reference local service accounts. Instead the `serviceAccount` field is replaced with a `serviceAccountRef` field which is an object reference to a service account in any namespace.

```yaml
apiVersion: kpack.io/v1alpha2
kind: ClusterBuilder
metadata:
  name: my-cluster-builder
spec:
  tag: gcr.io/sample/builder
  stack:
    name: bionic-stack
    kind: ClusterStack
  store:
    name: sample-cluster-store
    kind: ClusterStore
  serviceAccountRef:
    name: default
    namespace: default
  order:
  - group:
    - id: paketo-buildpacks/java
  - group:
    - id: paketo-buildpacks/nodejs
  - group:
    - id: kpack/my-custom-buildpack
      version: 1.2.3
    - id: kpack/my-optional-custom-buildpack
      optional: true
```

* `serviceAccountRef`: An object reference to a service account in any namespace. The object reference must contain `name` and `namespace`.

### <a id='order'></a>Order

The `spec.order` is cloud native buildpacks [builder order](https://buildpacks.io/docs/reference/builder-config/) that contains a list of buildpack groups.

This list determines the order in which groups of buildpacks will be tested during detection. Detection is a phase of the buildpack execution where buildpacks are tested, one group at a time, for compatibility with the provided application source code. The first group whose non-optional buildpacks all pass detection will be the group selected for the remainder of the build.

- **`group`** _(list, required)_\
  A set of buildpack references. Each buildpack reference specified has the following fields:

    - **`id`** _(string, required)_\
      The identifier of a buildpack from the configuration's top-level `buildpacks` list. This buildpack *must* be in available in the referenced store.

    - **`version`** _(string, optional, default: inferred)_\
      The buildpack version to chose from the store. If this field is omitted, the highest semver version number will be chosen in the store.

    - **`optional`** _(boolean, optional, default: `false`)_\
      Whether or not this buildpack is optional during detection.

> Note: Buildpacks with the same ID may appear in multiple groups at once but never in the same group.
