# Stacks

A stack resource is the specification for a [cloud native buildpacks stack](https://buildpacks.io/docs/concepts/components/stack/) used during build and in the resulting app image.
 
The stack will be referenced by a [builder](builders.md) resource. 
 
At this time only a Cluster scoped `ClusterStack` is available. 

### <a id='cluster-store'></a>Cluster Stack Configuration

```yaml
apiVersion: kpack.io/v1alpha1
kind: ClusterStack
metadata:
  name: base
spec:
  id: "io.buildpacks.stacks.bionic"
  buildImage:
    image: "paketobuildpacks/build:base-cnb"
  runImage:
    image: "paketobuildpacks/run:base-cnb"
```

* `id`:  The 'id' of the stack
* `buildImage.image`: The build image of stack.   
* `runImage.image`: The run image of stack.

### Updating a stack

The stack resource will not poll for updates. A CI/CD tool is needed to update the resource with new digests when new images are available.

### Suggested stacks

The [pack CLI](https://github.com/buildpacks/pack) command: `pack stack suggest` will display a list of recommended stacks that can be used. We recommend starting with the `io.buildpacks.stacks.bionic` base stack. 
  
