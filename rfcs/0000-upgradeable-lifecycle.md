**Problem:**
Currently, the lifecycle version used in kpack is not configurable, nor is it user upgradeable. Users must wait for a new kpack release to be able to upgrade the version of the lifecycle used by their builders.

**Outcome:**
We would like to decouple the lifecycle upgrade process from the kpack upgrade process, allowing users to upgrade the lifecycle on their own.

**Actions to take:**
We would create a ClusterLifecycle resource similiar a store or stack. The resources would be tied to each builder so that different builders can have different lifecycle versions. This has the benefit of allowing users to test new lifecycle versions without having to spin up another cluster. Also, users who created their own buildpack would not have to worry about a newer lifecycle dropping support for the Buildpack Api they are using. Due to the operator focused nature of this resource, and its similarity to ClusterStores and ClusterStacks, implementing this as a cluster scoped resource makes more sense at this time.

On the kp side, we could create commands that would assist users in uploading or relocating the lifecycle to their registry as well as managing existing lifecycles like they do with ClusterStores or ClusterStacks:
	
`kp clusterlifecycle create my-lifecycle --image "https://github.com/buildpacks/lifecycle/releases/download/v0.9.2/lifecycle-v0.9.2+linux.x86-64.tgz" --tag "gcr.io/my-registry/lifecycle:v0.9.2"`  
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

**Complexity:**
The first approach is less complex than the second, but could limit our options to change things down the road. The second option is definitely more complex, but follows the same pattern that we have for stores and stacks.

**Prior Art:**
* Any issues? Previous Ideas? Other Projects?

**Alternatives:**
A simpler, monolithic approach that would keep a single lifecycle version for the whole cluster like it is today, but with the functionality fo configuring that version exposed. The main benefit of soing something like that it will likely be the least intrusive on user workflow. The lifecycle would continue to be something that is more back of mind for users. There are multiple ways to handle this approach:

1. Create a command in `kp` (i.e `kp lifecycle upgrade`) that would give users the ability to manually update their lifecycle
2. Provide a script similiar to our `hack/lifecycle.go` that users can run 
3. Add the concept of lifecycle version to our descriptor files so that `kp import` can upgrade the lifecycle.

**Risks:**
Right now, the lifecycle is pretty hidden from the end user. In both these approaches, we would be making the user responsible for it, which could create extra work from them on the ci/testing front.