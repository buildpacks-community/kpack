`[Readable](https://github.com/your-name-or-org/kpack/blob/<pr-branch-name>/rfcs/0000-<my-feature>.md)`

**Problem:**

The CNB lifecycle is provided to kpack as in image reference in a configMap similar to how the completion and prepare
images are passed to the kpack controller. This means that every builder will always use the same exact lifecycle, with
no ability to specify an alternate one that may support different buildpack api or platform api versions. Due to kpack
support of windows, a custom lifecycle image with specific layers for the linux and windows binaries is required
(with a label specifying the layer diff id for each os). Because of this, a cve present in the lifecycle that has been 
patched upstream, requires any user to build the kpack specific lifecycle image themselves if the kpack provided one
has not been bumped.

As we [move to deprecate windows](https://github.com/buildpacks-community/kpack/discussions/1366), there is an opportunity for us to utilize
the upstream lifecycle image shipped by CNB, but switching to that image without changing the interface for providing the
lifecycle to kpack may open the door to incompatibilities if the wrong image is used on a particular kpack version.

**Outcome:**

Users will be able to create a CR that references a lifecycle image. The file structure in the image must match 
that of the [CNB Lifecycle image](https://hub.docker.com/r/buildpacksio/lifecycle/tags).

```
.
└── cnb/
    └── lifecycle/
        ├── analyzer
        ├── builder
        ├── detector
        ├── exporter
        ├── launcher
        ├── lifecycle.toml
        └── restorer
```

They will be able to use this lifecycle CR in any builder. A lifecycle will be shipped out of the box in kpack and 
users will not be required to provide a lifecycle reference in the builder if they do not wish to override the default.
Users should see no difference in functionality compared to the existing kpack lifecycle if they do not choose to 
provide their own lifecycle CR.

**Actions to take:**

A new ClusterLifecycle CRD will be created. It will be very similar in structure to the ClusterStack Resource. 
One open question is whether we should follow the Clusterstack model and keep this resource strictly cluster scoped or
follow the ClusterBuildpack pattern and provide a namespaced scoped version of the CRD. 

This is an example instance of the proposed CRD:
```yaml
apiVersion: kpack.io/v1alpha2
kind: ClusterLifecycle
metadata:
  name: sample-cluster-lifecycle
spec:
  serviceAccountRef:
    name: sample-sa
    namespace: sample-namespace
  image: buildpacksio/lifecycle@sha256:f4ce143ea6bbc6b5be5f4d151aba8214324adb93bbd7e3b1f43cd134ad450bf7
```

If we were to create a namespaced version of this CRD it would look like this. 
Similar to Buildpack CRs, ClusterBuilders will not be able to use the namespaced CR:
```yaml
apiVersion: kpack.io/v1alpha2
kind: Lifecycle
metadata:
  name: sample-cluster-lifecycle
  namespace: test
spec:
  serviceAccountName: sample-sa
  image: buildpacksio/lifecycle@sha256:f4ce143ea6bbc6b5be5f4d151aba8214324adb93bbd7e3b1f43cd134ad450bf7
```

The existing lifecycle reconciler can be updated to reconcile this new CRD instead of ConfigMaps.

A new field will be added to the ClusterBuilder/Builder resources allowing the user to provide a reference to a lifecycle CR.
This field will be optional and will default to the highest semantic version that is supported by the buildpacks in the builder and the platform. 
This is similar to how we determine the platform api to use for a build.
```yaml
apiVersion: kpack.io/v1alpha2
kind: Builder
metadata:
  name: my-builder
spec:
  tag: gcr.io/sample/builder
  serviceAccountName: default
  stack:
    name: stack
    kind: ClusterStack
  lifecycle:
    name: my-lifecycle
    kind: ClusterLifecycle
  order:
  - group:
    - id: paketo-buildpacks/java
```

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
  lifecycle:
    name: my-lifecycle
    kind: ClusterLifecycle
  serviceAccountRef:
    name: default
    namespace: default
  order:
  - group:
    - id: paketo-buildpacks/java
```

Creating a CRD for the lifecycle will allow us to do the following:
- Provide status resource that shows more information about the lifecycle such as supported api versions
- Allow users to specify a particular lifecycle for their builder, mimicking behavior that currently exists in the pack cli
- Offer a clean shift to using the upstream lifecycle image if done in conjunction with deprecation of windows support

**Complexity:**

The complexity of this is not that high due to its similarity with the other kpack resources.

**Prior Art:**

- The pack cli allows a lifecycle image to be specified when creating a build
- Adding new resources to the builder that get added to a layer has been done before in kpack

**Alternatives:**

We could keep the existing experience.

**Risks:**
Exposing the user to the lifecycle can result in some confusion, but this can be mitigated by not making it a required 
field on the builder and continuing to ship a lifecycle out of the box.