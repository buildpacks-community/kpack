## Problem:

The current v1alpha1/v1alpha2 Builders and ClusterBuilders are configured with an explicit ClusterStore as the "repository" of buildpacks available to construct the order specified in `.spec.order`. The ClusterStore is a kpack resource that provides a list of buildpackage images in `.spec.sources` and is not analogous to any other concept within the CNB ecosystem.

The ClusterStore being the single location for all buildpack images used in multiple builders makes managing the available buildpacks within kpack cumbersome. Adding new buildpacks requires modifying an existing resource and removing buildpacks requires carefully selecting the buildpack image to remove from the list of buildpackage images. The `kp` cli was built to handle this complexity but, the `kp` cli should not be a prerequisite to managing buildpacks within kpack.

The ClusterStore being a monolithic specification of all available buildpacks leads to performance issues within kpack. If the list of available buildpacks within a ClusterStore is lengthy the reconciliation of a single ClusterStore can take considerable time. If a single buildpack within the ClusterStore is unavailable reconciliation will fail which will cause the entire contents of the ClusterStore to be unavailable.

## Outcome:

Allow Buildpacks within kpack to be managed independently of other buildpacks with their own unique update cadence with a new ClusterBuildpack/Buildpack resource or directly within the (Cluster)Builder resource. This enables the ClusterStore to be eventually removed.

## Actions to take:

Introduce a new ClusterBuildpack and Buildpack resource that provides a buildpack to be available within a Cluster or a singular namespace and be used in ClusterBuilders and Builders.

Enable buildpacks to be used directly within (Cluster)Builders

##### Buildpack Resource

The Buildpack resource will allow for an individual buildpack to be configured with a single buildpackage image reference and included in a Builder with other Buildpacks versioned and managed independently.

```yaml
apiVersion: kpack.io/v1alpha3
kind: Buildpack
metadata:
  name: my-custom-app-buildpack-0.3.1
  namespace: my-kpack-using-namespace
spec:
  image: gcr.io/my-org/my-custom-app-buildpack:0.3.1
```

A Buildpack resource should be available to be used within all Builders within its namespace. A Buildpack should not be useable by any ClusterBuilder. Optional credentials should be provided to the Buildpack through a `serviceAccountName` reference.

```yaml
apiVersion: kpack.io/v1alpha3
kind: Buildpack
metadata:
  name: my-custom-app-buildpack-0.3.1
  namespace: my-kpack-using-namespace
spec:
  image: gcr.io/my-org/my-custom-app-buildpack:0.3.1
  serviceAccountName: my-service-account
```

The status of Buildpack resource should include the same metadata to the ClusterStore. This is to enable kpack to calculate the exact expected digest of the Builder image without needing to communicate with the registry. Additionally, this metadata may be used by external tooling such as the [kp cli](https://github.com/pivotal/kpack). The status must include the id, buildpack api, diffID, digest, size, homepage, supported stacks, and order if a provided buildpack is a [meta-buildpacks](https://buildpacks.io/docs/concepts/components/buildpack/#meta-buildpack) of each buildpack within the configured buildpackage image.

```yaml
apiVersion: kpack.io/v1alpha3
kind: Buildpack
metadata:
  name: my-custom-app-buildpack-0.3.1
  namespace: my-kpack-using-namespace
spec:
  image: gcr.io/my-org/my-custom-app-buildpack:0.3.1
status:
  conditions:
  - status: "True"
    type: Ready
  buildpacks:
  - api: "0.6"
    diffId: sha256:05aed3622d52837de1b1950069f6cca10aa3dbf7429e5c0497bae8cee2e6d901
    digest: sha256:9645b297c4c40b8476a243ae3c1ac9868986f64a000f9b95670ca9659428cec2
    homepage: https://github.com/my-org/my-custom-app-buildpack
    id: my-org/my-custom-app-buildpack
    order:
    - group:
      - id: my-org/some-custom-implementation-buildpack
        version: 1.4.0
    size: 3342595
    stacks:
    - id: io.buildpacks.stacks.bionic
    version: 4.2.2
  - api: "0.6"
    diffId: sha256:8e5c411f033cc771555aacbcb605cdbae7e3ae051a89515c4c278d0f704bd4ce
    digest: sha256:62812731bf5264f78b93c024adcfa5942929bdc07357041f70ca5d360d96bc5c
    id: my-org/some-custom-implementation-buildpack
    size: 3373979
    stacks:
    - id: io.buildpacks.stacks.bionic
    version: 1.4.0
```


##### Cluster Buildpack Resource

```yaml 
apiVersion: kpack.io/v1alpha3
kind: ClusterBuildpack
metadata:
  name: paketo-java-buildpack-5.9.1
spec:
  image: gcr.io/paketo-buildpacks/java:5.9.1
``` 

A ClusterBuildpack should be available to be used within any ClusterBuilder and Builders in any namespace. Optional credentials  should provided to the ClusterBuildpack through a service account reference

```yaml
apiVersion: kpack.io/v1alpha3
kind: ClusterBuildpack
metadata:
  name: paketo-java-buildpack-5.9.1
spec:
  image: privateregistry.com/paketo-buildpacks/java:5.9.1
  serviceAccountRef:
    name: private-sa
    namespace: private-ns
```

The status of the ClusterBuildpack should match the status of the Buildpack resource.

##### ClusterStore Resources

With the ClusterBuildpack and Buildpack resource, the ClusterStore resource will not be necessary and with the api that introduces the Buildpack Resource, the ClusterStore object reference should be removed on Builder resources and the ClusterStore should not be included within the api version.

##### Builder Resources

(Cluster)Builder resources do not need to explicitly reference Buildpack resources.

An example Builder without a configured ClusterStore is shown below.

```yaml
apiVersion: kpack.io/v1alpha3
kind: Builder
metadata:
  name: my-builder
spec:
  tag: gcr.io/sample/builder
  stack:
    name: bionic-stack
    kind: ClusterStack
  order:
  - group:
    - id: paketo-buildpacks/java      
```
The `paketo-buildpacks/java` buildpack referenced in the Builder order would be sourced from the Buildpack resource within the `my-builder` namespace or the ClusterBuildpack resource that provides the `paketo-buildpacks/java` with the highest version number as defined by semver. Buildpacks IDs defined in the builder order may be a "implementation buildpack" within a meta-buildpack. If multiple Buildpack resources provide the same selected Buildpack the Builder should reconcile with an error.

Builders should also be able to reference a Buildpack resource directly with an object reference within their order definition. An example Builder is shown below with an object reference within its order.

```yaml
apiVersion: kpack.io/v1alpha3
kind: Builder
metadata:
  name: my-builder
spec:
  tag: gcr.io/sample/builder
  stack:
    name: bionic-stack
    kind: ClusterStack
  order:
  - group:
    - id: paketo-buildpacks/java
    - name: my-custom-app-buildpack-0.3.1
      kind: Buildpack
```

Builders may also reference a buildpack within a buildpack resource and not necessarily the entire buildpackage. This is possible by providing an ID alongside an object reference.

```yaml
apiVersion: kpack.io/v1alpha3
kind: Builder
metadata:
  name: my-builder
spec:
  tag: gcr.io/sample/builder
  stack:
    name: bionic-stack
    kind: ClusterStack
  order:
  - group:
    - name: my-custom-app-buildpack-0.3.1
      kind: Buildpack
      id: my-custom/buildpack-inside
```

Alternatively, If users do not desire the complexity of external (Cluster)Buildpack resources, buildpackage images can be referenced directly within the Builder order. This removes the need to extenalize buildpack references from the Builder resource:

```yaml
apiVersion: kpack.io/v1alpha3
kind: Builder
metadata:
  name: my-builder
spec:
  tag: gcr.io/sample/builder
  stack:
    name: bionic-stack
    kind: ClusterStack
  order:
  - group:
    - image: gcr.io/paketo-buildpacks/java:5.9.1
    - name: my-custom-app-buildpack-0.3.1
      kind: Buildpack
```


**Prior Art:**
* [kapp controller](https://carvel.dev/kapp-controller/) models available Package versions as independent Package resources.

**Risks:**
This will require introducing a new api version within kpack. Future kpack releases will need to maintain support for ClusterStore resources until the kpack apis which contain the ClusterStore are removed.


**Appendix:**


##### Risk of not introducing a distinct resource

Instead of externalizing the specification of Buildpacks in a different resource from the Builder or ClusterBuilder the builder order could reference buildpack images and the ClusterStore could be deprecated. This would allow the ClusterStore to be removed and the creation of some builders would be simplified. However, this approach alone has a few notable drawbacks.

It is common for a buildpack to be used within multiple Builders or ClusterBuilders on a kpack cluster. Externalizing the specification of a Buildpack location within one resource on the Cluster removes unnecessary duplication. Updating a buildpack can occur with the create/update of a single resource which will propagate out to multiple Builder resources on a cluster.

Buildpacks have independent lifecycles from other buildpacks. A Buildpack resource allows operators of a kpack install to update a singular buildpack by modifying or creating a simple Buildpack resource. Without an external resource representing the Buildpack, the Builder resource would need to be updated despite having a distinct lifecycle from the Buildpack.

If Buildpack image locations are coupled to the Builder, Buildpack authors can not publish or release kpack configurations alongside buildpack releases without also releasing specifications for stack and allowing templating for the `.spec.tag`. This also prevents an individual buildpack kpack configuration from being packaged independently as a carvel package or helm package.

Some kpack clusters may be operated in an environment where different personas manage available buildpacks from the persona that manages the builder config. This is especially likely in an enterprise environment where only blessed buildpacks are available to developers but, developers can modify their buildpack usage by owning namespace scoped Builders. This can be restricted via RBAC with a distinct Buildpack resource but, would not be possible if Builders directly referenced buildpack images as developers could then use any Buildpack.

These distinct personas may also exist in non-enterprise settings where buildpacks and builders have independent lifecycles. For example, the kpack build cluster is a multitenant kpack cluster that provides up-to-date dependencies for multiple teams by continually updating new paketo dependencies. The kpack components are built with a kpack specific namespace Builder that utilizes the [Paketo Go Buildpack](https://github.com/paketo-buildpacks/go) alongside a custom buildpack to build libgit2. The kpack namespace scoped Builder continually pulls in new versions of the Paketo Go buildpack. If the buildpack images were configured on the Builder resource the process that updated the Paketo Go Buildpack would also need to update the kpack Custom Builder. This would be unnecessarily complicated and may not even be possible if the cluster operator does not know all the uses of a Buildpack within the cluster. 
