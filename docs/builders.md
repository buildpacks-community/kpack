# Builders

kpack provides Builder and ClusterBuilder resources to define and create [Cloud Native Buildpacks builders](https://buildpacks.io/docs/using-pack/working-with-builders/) all within the kpack api.
This allows granular control of how stacks, buildpacks, and buildpack versions are utilized and updated.

Before creating Builders you will need to create a [ClusterStack](stack.md) and
either [Buildpack](buildpacks.md#buildpack), [ClusterBuildpack](buildpacks.md#cluster-buildpack), or
[ClusterStore](buildpacks.md#cluster-store) resources below.

> Note: The Builder and ClusterBuilder were previously named CustomBuilder and
> CustomClusterBuilder. The previous Builder and ClusterBuilder resources that
> utilized pre-built builders were removed and should no longer be used with
> kpack. This was discussed in an approved
> [RFC](https://github.com/pivotal/kpack/pull/439).

Corresponding `kp` cli command docs for
[builders](https://github.com/vmware-tanzu/kpack-cli/blob/main/docs/kp_builder.md)
and [cluster builders](https://github.com/vmware-tanzu/kpack-cli/blob/main/docs/kp_clusterbuilder.md).

### <a id='builders'></a>Builders

The Builder uses a [ClusterStack](stack.md), an optional [ClusterStore](buildpacks.md#cluster-store), and an order definition to construct a builder image.

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
    - id: kpack/my-custom-buildpack
      version: 1.2.3
    - id: kpack/my-optional-custom-buildpack
      optional: true
  - group:
    - name: sample-buildpack
      kind: Buildpack
    - name: sample-cluster-buildpack
      kind: ClusterBuildpack
      id: paketo-buildpacks/nodejs
      version: 1.2.3
```

* `tag`: The tag to save the builder image. You must have access via the referenced service account.
* `serviceAccount`: A service account with credentials to write to the builder tag.
* `order`: The [builder order](https://buildpacks.io/docs/reference/builder-config/). See the [Order](#order) section below.
* `stack.name`: The name of the stack resource to use as the builder stack. All buildpacks in the order must be compatible with the clusterStack.
* `stack.kind`: The type as defined in kubernetes. This will always be ClusterStack.
* `store`: If using ClusterStore, then the reference to the ClusterStore. See the [Resolving Buildpack IDs](#resolving-buildpack-ids) section below.
  * `name`: The name of the ClusterStore resource in kubernetes.
  * `kind`: The type as defined in kubernetes. This will always be ClusterStore.

### <a id='cluster-builders'></a>Cluster Builders

The ClusterBuilder resource is almost identical to a Builder but, it is a
cluster scoped resource that can be referenced by an image in any namespace.
Because ClusterBuilders are not in a namespace they cannot reference local
service accounts. Instead the `serviceAccount` field is replaced with a
`serviceAccountRef` field which is an object reference to a service account in
any namespace.

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
    - id: kpack/my-custom-buildpack
      version: 1.2.3
    - id: kpack/my-optional-custom-buildpack
      optional: true
  - group:
    - name: sample-cluster-buildpack
      kind: ClusterBuildpack
      id: paketo-buildpacks/nodejs
    - id: paketo-buildpacks/nodejs # can obtain buildpacks from a Store/ClusterStore
      version: 1.2.3
```

* `serviceAccountRef`: An object reference to a service account in any namespace. The object reference must contain `name` and `namespace`.

### <a id='order'></a>Order

The `spec.order` is cloud native buildpacks [builder order](https://buildpacks.io/docs/reference/builder-config/)
that contains a list of buildpack groups.

This list determines the order in which groups of buildpacks will be tested
during detection. Detection is a phase of the buildpack execution where
buildpacks are tested, one group at a time, for compatibility with the provided
application source code. The first group whose non-optional buildpacks all pass
detection will be the group selected for the remainder of the build.

- **`group`** _(list, required)_\
  A set of buildpack references. Each buildpack reference specified is one of the following:
  - A kubernetes object reference:
    - **`kind`** _(string, required)_\
      The kubernetes kind, must be either `Buildpack` (Builder only), or
      `ClusterBuildpack`.

    - **`name`** _(string, required)_\
      The name of the kubernetes object.

  - Buildpack info:
    - **`id`** _(string, required)_\
      The identifier of a buildpack from the configuration's top-level
      `buildpacks` list. For rules on which resource the buildpack is resolved
      from, see [Resolving BuildpackIDs](#resolving-buildpack-ids) section
      below.

    - **`version`** _(string, optional, default: inferred)_\
      The buildpack version to chose from the store. If this field is omitted,
      the highest semver version number will be chosen in the store.

    - **`optional`** _(boolean, optional, default: `false`)_\
      Whether or not this buildpack is optional during detection.

  - Both object reference and buildpack info together.

> Note: Buildpacks with the same ID may appear in multiple groups at once but never in the same group.

### <a id='resolving-buildpacks-ids'></a>Resolving Buildpack IDs

When using the kubernetes object reference with buildpack info, kpack will try
to locate the specified buildpack and version within the resource. If using
Buildpack resources, it must be within the same namespace as the Builder.

When using the kubernetes object reference without any buildpack info, the
"root" buildpack will be chosen. A "root" buildpack is any buildpack that is
not specified in the order of another buildpack. If there are multiple "root"
buildpacks, the buildpack resource will fail to reconcile.

When using just the buildpack info, kpack will try to find the buildpack (and
version) in the following order:

1. As a sub-buildpack of any Buildpacks in the Builder's namespace (i.e.
   `paketo-buildpacks/gradle` is a sub-buildpack of `paketo-buildpacks/java`)
1. As a sub-buildpack of any ClusterBuildpacks
1. As a sub-buildpack in the ClusterStore specified in the Builder spec.

### <a id='status'></a>Builder Status Conditions

Builders and ClusterBuilders have two Conditions that represent the overall status of the Builder.

The 'Ready' condition is used to show that the Builder is able to be used in Builds

The 'UpToDate' condition indicates whether the most recent reconcile of the Builder 
was successful. When this condition is false, that means that the Builder may not have the 
latest Stack or Buildpacks due to ongoing reconcile failures.