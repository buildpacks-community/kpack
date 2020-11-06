## Problem:

kpack does not support building images that can run on windows

## Outcomes:

### kpack should run on clusters that have windows workers

kpack service pods and build pods for linux images will need to know not to run on the windows workers to work in a mixed-os cluster.

The kpack release configuration should specify to run pods explicitly on linux os.

The kpack controller should schedule linux builds on linux workers.

**Actions to Take**

Use the following node selector for all service pods and build pods for linux images:

```
nodeSelector:
    kubernetes.io/os: linux
```

### kpack should support creating windows builders

This is currently supported in the [pack cli](https://github.com/buildpacks/pack/issues/469)

**Actions to Take**

- kpack-controller will use a new environment variable `WINDOWS_LIFECYCLE_IMAGE` to reference the windows lifecycle image
- Provide the kpack release (`release.yaml`) with a windows version of the lifecycle image and configure it with `WINDOWS_LIFECYCLE_IMAGE`
- When a Builder or ClusterBuilder is created using a windows stack, kpack-controller must create a windows builder using the windows lifecycle image.
    - Image os is determined by the image [os property](https://github.com/opencontainers/image-spec/blob/master/config.md#:~:text=os%20string).

Notes:

- Builders will not be aware of the buildpackages os. The only compatibility constraint between stacks and buildpackages for creating builders will be the stack id.

### kpack should support running builds on windows workers

**Actions to Take**

- kpack-controller will accept new configuration via environment variable(s) for windows image(s) needed to run builds. The current behavior that must be recreated on windows is contained in the following images:
    - `BUILD_INIT_IMAGE` - getting the build ready with creds and source
    - `COMPLETION_IMAGE` - an image to run as a required regular (non-init) container
    - We'll call these "helper images"
    - Out of scope:
        - What to name the windows versions of these images
        - How to configure the environment variables (whether it should be one image or multiple)
- Provide the windows helper images with kpack release (`release.yaml`) for Windows Server 2019 LTSC aka version 1804
- When a Build is created with a builder that has an image label of `os: windows`, it will
    - Use the windows helper images for the respective containers of the build pod
    - Use the following node selector for the build pod

 ```
 nodeSelector:
     kubernetes.io/os: windows
 ```

## Complexity:

High

## Prior Art:

[pack cli](https://github.com/buildpacks/pack) has support for windows containers
[example windows stack dockerfile](https://github.com/buildpacks/samples/blob/main/stacks/dotnet-framework-1809/build/Dockerfile)

## Risks:

**Windows images can only run and be created on workers with the same OS version**

Windows images can only run on windows nodes with the same OS. ex: Windows Server LTSC 2019 can only run images that are ltsc2019 or equivalent version [docs](https://docs.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility?tabs=windows-server-2019%2Cwindows-10-20H2#:~:text=Supports%20process%20isolation)

Ergo:

- Each windows helper image installed with `kpack` must be for the same os version and must match the cluster's windows nodes
    - We will be providing these images for Windows Server 2019 LTSC
- OCI images created on windows nodes will match the os version of the node

Effectively we will only support Windows Server 2019 LTSC. Handling stacks and nodes that do not use this version is outside of the scope of the rfc, mitigated with docs.

Notes:
- Stores will not be aware of the buildpackages os. There will be no constraint on creating stores with windows buildpackages and linux buildpackages.

[Supported windows versions](https://kubernetes.io/docs/setup/production-environment/windows/intro-windows-in-kubernetes/#windows-os-version-support)

Note:
- Windows Server 2019 (LTSC) aka version 1809 is the only Windows operating system supported currently on all k8s api versions [see](https://kubernetes.io/docs/setup/production-environment/windows/intro-windows-in-kubernetes/#windows-containers-in-kubernetes)
    - Some SAC versions are supported on Kubernetes v1.18 and Kubernetes v1.19 ([docs](https://kubernetes.io/docs/setup/production-environment/windows/intro-windows-in-kubernetes/#windows-os-version-support))
- Once they support Windows containers with [Hyper-V isolation](https://kubernetes.io/docs/setup/production-environment/windows/intro-windows-in-kubernetes/#hyper-v-isolation) in Kubernetes, the limitation and compatibility rules will change.

**Some background around windows versioning:**

Windows has a Long Term Support Channel (LTSC, 5 years support) and a Semi-Annual Channel (SAC, 18 months support)

Windows versions can be referenced by either of 2 numbers:
- **Version** which uses the year and month it was released (ex 1903, 1909, 2004)
- **Build Number** which is just another way to reference the version (ex: `10.0.17763.1518` format: `<major>.<minor>.<build>.<revision>`)
    - Only matching `<major>.<minor>.<build>` are compatible, `revision` can be bumped with security patches

The current LTSC is usually referenced as "Windows Server 2019" (the actual version is 1809 aka build 10.0.17763)

K8s supports Windows Server 2019 on all k8s versions and SACs per k8s version minor, see [docs](https://kubernetes.io/docs/setup/production-environment/windows/intro-windows-in-kubernetes/#windows-os-version-support).

**If there are no windows nodes, windows builds will not run**

Handling this edge case is outside the scope of the rfc, mitigated with docs.

**Windows stacks may have different ARCH affecting compatibility with kpack**

A windows stack may support x64, ARM or others and may not be compatible with the windows nodes or kpack "helper images"

This concern is considered out of scope of the rfc.

## Alternatives:

**Restrict Stores to only allow same-os buildpackages**

This could help with maintenance of stores but is not technically necessary
