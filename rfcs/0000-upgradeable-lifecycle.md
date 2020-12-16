**Problem:**
Currently, the lifecycle version used in kpack is not configurable, nor is it user upgradeable. Users must wait for a new kpack release to be able to upgrade the version of the lifecycle used by their builders.

**Outcome:**
We would like to decouple the lifecycle upgrade process from the kpack upgrade process, allowing users to upgrade the lifecycle on their own.

**Actions to take:**
A simple, monolithic approach that would keep a single lifecycle version for the whole cluster like it is today, but with the functionality fo configuring that version exposed. The main benefit of doing something like this it will likely be the least intrusive on user workflow. The lifecycle would continue to be something that is more back of mind for users. We would ship a version of the lifecycle with each kpack release like we do today. Additionally, since kpack has to be compatible with the platform api of the lifecycle, keeping the lifecycle version tied to kpack instead of a resource on a builder seems to make more sense right now. We would implement this by adding a few features to kp:

1. Create a command in `kp` that would give users the ability to manually update their lifecycle:

	`kp lifecycle upgrade --path "https://github.com/buildpacks/lifecycle/releases/download/v0.9.2/lifecycle-v0.9.2+linux.x86-64.tgz"`  
	or  
	`kp lifecycle upgrade --image "gcr.io/my-registry/lifecycle:v0.9.2"`  

2. Add the concept of lifecycle version to our descriptor files so that `kp import` can upgrade the lifecycle. 

We would also need compatibility checking to make sure that kpack can support the version of the lifecycle that is being used. Additionally, we would not want all images to be rebuilt on a lifecycle upgrade. 

One issue with this approach is that currently, the lifecycle image is passed to the kpack controller as an environment variable, so any change to the lifecycle image would require a kpack controller restart to propogate that change. We could get around this by introducing a `kpack-config` config map that would be monitored by the kpack controller for changes. To start, this config will probably just have the lifecycle version in it, but it does open the door for us to add other config options further down the road. An example `kpack-config` could look like:

```yaml

apiVersion: v1
data:
  lifecycle.image: gcr.io/my-registry/lifecycle:v0.9.2
kind: ConfigMap
metadata:
  name: kpack-config
  namespace: kpack

```

**Complexity:**
The proposed approach is less complex than the alternative, but could limit our options to change things down the road. The alternative option is definitely more complex, but follows the same pattern that we have for stores and stacks. The main point of complexity for the proposed solution is figuring out how to update the lifecycle without requiring the kpack controller to be restarted.

**Prior Art:**
* Any issues? Previous Ideas? Other Projects?

**Alternatives:**
We would create a ClusterLifecycle resource similiar a store or stack. The resources would be tied to each builder so that different builders can have different lifecycle versions. This has the benefit of allowing users to test new lifecycle versions without having to spin up another cluster. Also, users who created their own buildpack would not have to worry about a newer lifecycle dropping support for the Buildpack Api they are using. Due to the operator focused nature of this resource, and its similarity to ClusterStores and ClusterStacks, implementing this as a cluster scoped resource makes more sense at this time.

On the kp side, we could create commands that would assist users in uploading or relocating the lifecycle to their canonical registry as well as managing existing lifecycles like they do with ClusterStores or ClusterStacks:
	
`kp clusterlifecycle create my-lifecycle --path "https://github.com/buildpacks/lifecycle/releases/download/v0.9.2/lifecycle-v0.9.2+linux.x86-64.tgz"
or   
`kp clusterlifecycle create my-lifecycle --image "gcr.io/my-registry/lifecycle:v0.9.2"`

The spec for a lifecycle resource could look something like this:

```yaml

apiVersion: kpack.io/v1alpha1
kind: ClusterLifecycle
metadata:
  name: default
spec:  
  image: "gcr.io/some-registry/lifecycle:v0.9.2"
```
	
This approach would also allow us to add lifecycles to descriptor files for easier updating.
	
On the (Cluster)Builder side of things, we could modify the spec to add a `lifecycle` field:
	

```yaml

apiVersion: kpack.io/v1alpha1
kind: Builder
metadata:
  name: some-builder
  namespace: default
spec:
  serviceAccount: default
  tag: gcr.io/some-registry/builder
  lifecycle:
    name: default
    kind: ClusterLifecycle
  stack:
    name: default
    kind: ClusterStack
  store:
    name: default
    kind: ClusterStore
  order:
  - group:
    - id: paketo-buildpacks/ruby
  - group:
    - id: paketo-buildpacks/nodejs
  - group:
    - id: paketo-buildpacks/java
```
**Risks:**
Right now, the lifecycle is pretty hidden from the end user. In both these approaches, we would be making the user responsible for it, which could create extra work from them on the ci/testing front.
