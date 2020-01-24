# Experimental Custom Builders

kpack provides the experimental CustomBuilder and CustomClusterBuilder resources to define and create [Cloud Native Buildpacks builders](https://buildpacks.io/docs/using-pack/working-with-builders/) all within the kpack api. 
This allows more granular control of the buildpacks and buildpack versions utilized without relying on pre-existing Builder resources. 

Before creating CustomBuilders you will need to create the Stack and Store resources described below. 

### <a id='store'></a>Store

The Store is a cluster scoped resource resource is a repository for buildpacks that can be utilized by CustomBuilders. As an input the store takes a list of images that contain buildpacks.

```yaml
apiVersion: experimental.kpack.pivotal.io/v1alpha1
kind: Store
metadata:
  name: sample-store
spec:
  sources:
  - image: gcr.io/cf-build-service-public/node-engine-buildpackage@sha256:95ff756f0ef0e026440a8523f4bab02fd8b45dc1a8a3a7ba063cefdba5cb9493
  - image: gcr.io/cf-build-service-public/npm-buildpackage@sha256:5058ceb9a562ec647ea5a41008b0d11e32a56e13e8c9ec20c4db63d220373e33
  - image: cloudfoundry/cnb:bionic
```

* `sources`:  List of builder images or buildpackage images to make available in the store. Each image is an object with the key image.   
 
For kpack to use an image in the store it, the OCI image label 'io.buildpacks.buildpack.layers' must contain buildpacks and buildpack metadata (this label is viewable via docker inspect).
 
### <a id='store'></a>Stack

The Stack is a cluster scoped resource that provides the configuration for a [Cloud Native Buildpack stack](https://buildpacks.io/docs/concepts/components/stack/) that is available to be used in a Custom Builder.   

The [pack CLI](https://github.com/buildpacks/pack) command: `pack suggest-stacks` will display a list of recommended stacks that can be used. We recommend starting with the `io.buildpacks.stacks.bionic` stack. 

```yaml
apiVersion: experimental.kpack.pivotal.io/v1alpha1
kind: Stack
metadata:
  name: bionic-stack
spec:
  id: "io.buildpacks.stacks.bionic"
  buildImage:
    image: "cloudfoundry/build@sha256:b30b850f5b4d2e11313f0ec152952eace285ce7a3bc203ca5cdcfa8e5bb232a6"
  runImage:
    image: "cloudfoundry/run@sha256:ba9998ae4bb32ab43a7966c537aa1be153092ab0c7536eeef63bcd6336cbd0db"
```

* `id`:  The 'id' of the stack
* `buildImage.image`: The build image of stack.   
* `runImage.image`: The run image of stack.

> Note: The stack resource will not poll for updates. A CI/CD tool is needed to update the resource with new digests when new images are available.     

### <a id='store'></a>Custom Builders

The CustomBuilder uses a [Store](#store), a [Stack](#stack), and an order definition to construct a builder image.

```yaml
apiVersion: experimental.kpack.pivotal.io/v1alpha1
kind: CustomBuilder
metadata:
  name: my-custom-builder
spec:
  tag: gcr.io/sample/custom-builder
  serviceAccount: default
  stack: bionic-stack
  store: sample-store
  order:
  - group:
    - id: org.cloudfoundry.node-engine
    - id: org.cloudfoundry.yarn
  - group:
    - id: org.cloudfoundry.openjdk
    - id: org.cloudfoundry.buildsystem
      optional: true
    - id: org.cloudfoundry.jvmapplication 
```

* `tag`: The tag to save the custom builder image. You must have access via the referenced service account.   
* `serviceAccount`: A service account with credentials to write to the custom builder tag. 
* `order`: The [builder order](https://buildpacks.io/docs/reference/builder-config/). 
* `stack`: The name of the stack resource to use as the builder stack. All buildpacks in the order must be compatible with the stack.   
* `store`: The name of the store resource to fetch buildpacks from.

The custom builder can be referenced in an image configuration like this:

```yaml
builder:
  name: my-custom-builder
  kind: CustomBuilder
```

### <a id='store'></a>Custom Cluster Builders

The CustomClusterBuilder resource is almost identical to a CustomBuilder but, it is a cluster scoped resource that can be referenced by an image in any namespace. Because CustomClusterBuilders are not in a namespace they cannot reference local service accounts. Instead the `serviceAccount` field is replaced with a `serviceAccountRef` field which is an object reference to a service account in any namespace.    

```yaml
apiVersion: experimental.kpack.pivotal.io/v1alpha1
kind: CustomClusterBuilder
metadata:
  name: my-cluster-builder
spec:
  tag: sample/custom-builder
  stack: bionic-stack
  store: sample-store
  serviceAccountRef:
    name: default
    namespace: default
  order:
  - group:
    - id: org.cloudfoundry.node-engine
    - id: org.cloudfoundry.yarn
  - group:
    - id: org.cloudfoundry.node-engine
    - id: org.cloudfoundry.npm
```

* `serviceAccountRef`: An object reference to a service account in any namespace. The object reference must contain `name` and `namespace`.

The custom cluster builder can be referenced in an image configuration like this:

```yaml
builder:
  name: my-custom-builder
  kind: CustomBuilder
```