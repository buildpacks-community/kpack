## Problem:

kpack does not support building containers that can run on windows

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
- When a Builder or ClusterBuilder is created using a windows stack and windows buildpackages, kpack-controller must create a windows builder using the windows lifecycle image.
    - Image os is determined by the image [os property](https://github.com/opencontainers/image-spec/blob/master/config.md#:~:text=os%20string).

Notes:

- Builders with mixed os images must error gracefully
- Builders should be labeled with `os.kpack.io: windows` for os disambiguation independent of image metadata

### kpack should support running builds on windows workers

**Actions to Take**

- kpack-controller will accept new environment variables for windows images needed to run builds. These will have the same functionality as on linux and will be used for running builds on windows workers.
    - `WINDOWS_BUILD_INIT_IMAGE` - a windows build init image
    - `WINDOWS_COMPLETION_IMAGE` - a windows completion image
    - `WINDOWS_REBASE_IMAGE` - a windows rebase image
    - These will be optional to remove the dependency on windows images
- When an Image is created using a windows Builer or ClusterBuilder labeled with `os.kpack.io: windows`, it will create a build with new properties
    - The Build will use `WINDOWS_BUILD_INIT_IMAGE` and `WINDOWS_COMPLETION_IMAGE` for the respective containers.
    - The Build will be labeled with `os.kpack.io: windows` for os disambiguation
 - When a Build is created with the label `os.kpack.io: windows`, it will use the node selector

 ```
 nodeSelector:
     kubernetes.io/os: windows
 ```

## Complexity:

High

## Prior Art:

[pack cli](https://github.com/buildpacks/pack) has support for windows containers

## Alternatives:



## Risks:

**If there are no windows nodes, windows builds will not run**

This is probably out of scope for the rfc
