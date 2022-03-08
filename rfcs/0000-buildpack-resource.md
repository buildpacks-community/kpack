**Problem:**

The current v1alpha1/v1alpha2 Builders and ClusterBuilders are configured with an explicit ClusterStore as the "repository" of buildpacks available to construct the order specified in `.spec.order`. The ClusterStore is a kpack resource that provides a list of buildpackage images in `.spec.sources` and is not analogous to any other concept within the CNB ecosystem.

The ClusterStore being the single location for all buildpack images used in multiple builders makes managing the available buildpacks within kpack cumbersome. Adding new buildpacks requires modifying an existing resource and removing buildpacks requires carefully selecting the buildpack image to remove from the list of buildpackage images. The `kp` cli was built to handle this complexity but, the `kp` cli should not be a prerequisite to managing buildpacks within kpack.

The ClusterStore being a monolithic specification of all available buildpacks leads to performance issues within kpack. If the list of available buildpacks within a ClusterStore is lengthy the reconciliation of a single ClusterStore can take considerable time. If a single buildpack within the ClusterStore is unavailable reconciliation will fail which will cause the entire contents of the ClusterStore to be unavailable.

**Outcome:**

Allow Buildpacks within kpack to be managed independently of other buildpacks with their own unique lifecycles with a new ClusterBuildpack/Buildpack resource while eventually removing support for the ClusterStore.

**Actions to take:**

Introduce a new ClusterBuildpack and Buildpack resource that provides a buildpack to be available within a Cluster or a singular namespace and be used in ClusterBuilders and Builders.

The Buildpack resource will allow for an individual buildpack to be configured with a single image reference.

Sample Cluster Buildpack Resource

```yaml 
apiVersion: kpack.io/v1alpha3
kind: ClusterBuildpack
metadata:
  name: paketo-java-buildpack-5.9.1
spec:
  image: gcr.io/paketo-buildpacks/java:5.9.1
``` 

Sample Buildpack Resource

```yaml
apiVersion: kpack.io/v1alpha3
kind: Buildpack
metadata:
  name: my-custom-app-buildpack-0.3.1
spec:
  image: gcr.io/my-org/my-custom-app-buildpack:0.3.1
``` 

With the Buildpack resource, the ClusterStore resource will not be necessary and with the api that introduces the Buildpack Resource, the ClusterStore object reference should be removed on Builder resources.

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
The `paketo-buildpacks/java` buildpack referenced in the Builder order would be sourced from the Buildpack resource within the `my-builder` namespace or the ClusterBuildpack resource that provides the `paketo-buildpacks/java` with the highest version number as defined by semver. If multiple Buildpack resources provide the same selected Buildpack the Builder should reconcile with an error.

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

**Open Questions:**
* Should ClusterBuilders be able to use Buildpacks or only ClusterBuildpacks?


**Prior Art:**
* [kapp controller](https://carvel.dev/kapp-controller/) models available Package versions as indepdent Package resources.

**Risks:**
This will require introducing a new api version within kpack. Future kpack releases will need to maintain support for ClusterStore resources until the kpack apis which contain the ClusterStore are removed.

**Alternatives:**

##### Remove the ClusterStore without adding Buildpack Resources

Instead of externalizing the specification of Buildpacks in a different resource from the Builder or ClusterBuilder the builder order could reference buildpack images and the ClusterStore could be deprecated. This would allow the ClusterStore to be removed and the creation of some builders would be simplified. However, this approach alone has a few notable drawbacks.

It is common for a buildpack to be used within multiple Builders or ClusterBuilders on a kpack cluster. Externalizing the specification of a Buildpack location within one resource on the Cluster removes unnecessary duplication. Updating a buildpack can occur with the create/update of a single resource which will propagate out to multiple Builder resources on a cluster.

Buildpacks have independent lifecycles from other buildpacks. A Buildpack resource allows operators of a kpack install to update a singular buildpack by modifying or creating a simple Buildpack resource. Without an external resource representing the Buildpack, the Builder resource would need to be updated despite having a distinct lifecycle from the Buildpack.

If Buildpack image locations are coupled to the Builder, Buildpack authors can not publish or release kpack configurations alongside buildpack releases without also releasing specifications for stack and allowing templating for the `.spec.tag`. This also prevents an individual buildpack kpack configuration from being packaged independently as a carvel package or helm package.

Some kpack clusters may be operated in an environment where different personas manage available buildpacks from the persona that manages the builder config. This is especially likely in an enterprise environment where only blessed buildpacks are available to developers but, developers can modify their buildpack usage by owning namespace scoped Builders. This can be restricted via RBAC with a distinct Buildpack resource but, would not be possible if Builders directly referenced buildpack images as developers could then use any Buildpack.

These distinct personas may also exist in non-enterprise settings where buildpacks and builders have independent lifecycles. For example, the kpack build cluster is a multitenant kpack cluster that provides up-to-date dependencies for multiple teams by continually updating new paketo dependencies. The kpack components are built with a kpack specific namespace Builder that utilizes the [Paketo Go Buildpack](https://github.com/paketo-buildpacks/go) alongside a custom buildpack to build libgit2. The kpack namespace scoped Builder continually pulls in new versions of the Paketo Go buildpack. If the buildpack images were configured on the Builder resource the process that updated the Paketo Go Buildpack would also need to update the kpack Custom Builder. This would be unnecessarily complicated and may not even be possible if the cluster operator does not know all the uses of a Buildpack within the cluster. 
