# kpack
[![Build Status](https://github.com/pivotal/kpack/workflows/CI/badge.svg)](https://github.com/pivotal/kpack/actions)
[![codecov](https://codecov.io/gh/pivotal/kpack/branch/master/graph/badge.svg)](https://codecov.io/gh/pivotal/kpack)

Kubernetes Native Container Build Service

kpack extends [Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and utilizes unprivileged kubernetes primitives to provide builds of OCI images as a platform implementation of [Cloud Native Buildpacks](https://buildpacks.io) (CNB).

kpack provides a declarative builder resource that configures a Cloud Native Buildpacks build configuration with the desired buildpack order and operating system stack.

kpack provides a declarative image resource that builds an image and schedules image rebuilds on source changes and from builder buildpack and builder stack updates.

kpack also provides a build type to execute a single Cloud Native Buildpack image build.

![kpack gif](docs/assets/node-min.gif)

### Documentation & Getting Started

- [Install kpack](docs/install.md)
- Get started with [the tutorial](docs/tutorial.md) 
- Check out the documentation on kpack concepts:
    - [Stacks](docs/stack.md)
    - [Stores](docs/store.md)
    - [Builders](docs/builders.md)
    - [Images](docs/image.md)
    - [Secrets](docs/secrets.md)
    - [Builders](docs/builders.md)
    - [Builds](docs/build.md)
    - [Service Bindings](docs/servicebindings.md)

- Tailing logs with the kpack [log utility](docs/logs.md)
 
- Documentation on [Local Development](docs/local.md)
