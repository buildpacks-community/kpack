
# kpack vs pack

So, you are a [pack][_pack] user trying to learn about [kpack][_kpack] and get your [Cloud Native Buildpacks][_cnb] journey to the next level? then you are in the right place, on the next sections we are going to explain the similarities between [pack][_pack] and [kpack][_kpack].

First of all, both [kpack][_kpack] and [pack][_pack] implement the [platform interface](https://github.com/buildpacks/spec/blob/main/platform.md) [specification](https://github.com/buildpacks/spec/blob/main/platform.md), but they do it for two non-overlapping contexts: while [pack][_pack] targets developers and local builds, [kpack][_kpack] manages containerization on day-2 and at scale and is a [Kubernetes](https://kubernetes.io/) native implementation.

We will define some basic use case scenarios and see how we can get the output from both tools.

## Assumptions

In order to make our comparison very simple, lets make some assumptions:
1. Our application source code is one of the [samples](https://github.com/buildpacks/samples/tree/main/apps) application
2. We are going to use [Cloud Native Buildpacks](_cnb) [sample builder](https://hub.docker.com/r/cnbs/sample-builder) to build our application source code
3. We need **write** access to a remote registry to publish our application image

## Build scenario

Let's define our most basic use case as follows:

`As a [pack|kpack] user, I want to convert my application source code into an image and publish it into a remote registry`

### Pack Implementation

In order to build our application source code using [pack][_pack] we need to run a command similar to this:

`pack build --publish --path apps/<APP> --builder cnbs/sample-builder:<bionic OR alpine> <app-image-name>`

After building your '<app-image-name>' must be written into your remote registry.

### Kpack Implementation

How do we get a similar functionality to a `pack build` command using [kpack][_kpack]? the answer is the Build resource!

Once you have [kpack][_kpack] up and running on a kubernetes cluster, you need to create a Build resource and apply it to your cluster. for our scenario it looks like this:

```yaml
apiVersion: kpack.io/v1alpha2
kind: Build
metadata:
  name: sample-build # This can be any name
spec:
  tags:
    - <app-image-name>
  builder:
    image: cnbs/sample-builder:<bionic OR alpine>
  source:
    git:
      url: https://github.com/buildpacks/samples.git
      revision: main
    subPath: "apps/<APP>"
```

Once you create yaml file, the next step is just to apply the resource into your kubernetes cluster, for example using

```bash
kubectl apply -f <your-build-resource.yaml>
```

After building, your '<app-image-name>' must be also written into your remote registry.

**Note** Probably you will need to create some [secrets](secrets.md) to give [kpack][_kpack] access to your remote registry, but this is also required on [pack][_pack], so please check the documentation depending on your registry provider

## Re-build scenario

`As a [pack|kpack] user, I want to rebuild my application source code after some change and publish a new image into a remote registry`

### Pack Implementation

In [pack][_pack], in order to re-build your application image, you just need to run the `pack build` command after saving your application source code changes

### Kpack Implementation

As we mentioned above, All fields on a build are immutable, this mean that every time we want to run a build we must create a new `Build` resource. One way to do this is using `generateName` field in our resource definition.

From our previous resource definition, let's remove the `metadata.name` field, and replace it with a `metadata.generateName` this value will be used by the server, to generate a unique name ONLY IF the Name field has not been provided.

```yaml
apiVersion: kpack.io/v1alpha2
kind: Build
metadata:
  generateName: sample-build- # this value will be a prefix 
spec:
  tags:
    - <app-image-name>
  builder:
    image: cnbs/sample-builder:<bionic OR alpine>
  source:
    git:
      url: https://github.com/buildpacks/samples.git
      revision: main
    subPath: "apps/<APP>"
```
Once you create yaml file, any time you want to create a build, just run

```bash
kubectl create -f <your-build-resource.yaml>
```

A new build resource will be created and a unique suffix will be added to the value provided, for example: `sample-build-2vsz5`

Note: use `create` instead of `apply` when using `generateName`

## Rebase scenario

`As a [pack|kpack] user, I want to rebase my application image with a new run-image from the stack`

### Pack Implementation

[pack][_pack] offers the `pack rebase` command to accomplish this goal, for example:

```bash
pack rebase --publish <app-image-name>
```

### Kpack Implementation

A standalone build can be triggered to be rebase if [kpack][_kpack] detects it is a "rebase-able" build.

A build is considered "rebase-able" if the following conditions are met:

- the field `spec.lastBuild.stackId` is equal to <same-stack-id-as-the-builder>
- An annotation key `image.kpack.io/reason` is equal to `STACK`

An example resource that also is configured to use the `generateName` field could be as follows:

```yaml
apiVersion: kpack.io/v1alpha2
kind: Build
metadata:
  generateName: sample-build- # this value will be a prefix 
  annotations:
    image.kpack.io/reason: STACK
spec:
  lastBuild:
    stackId: <same-stack-id-as-the-builder>
  tags:
    - <app-image-name>
  builder:
    image: cnbs/sample-builder:<bionic OR alpine>
  source:
    git:
      url: https://github.com/buildpacks/samples.git
      revision: main
    subPath: "apps/<APP>"
```
Once you create yaml file, just run

```bash
kubectl create -f <your-build-resource.yaml>
```

[kpack][_kpack] will create a pod execution the rebase operation


[_pack]:https://github.com/buildpacks/pack
[_kpack]:https://github.com/pivotal/kpack
[_cnb]:https://buildpacks.io
