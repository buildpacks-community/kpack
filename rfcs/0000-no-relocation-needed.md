**Problem:**

When a Builder or ClusterBuilder is written to its `.spec.tag` the kpack controller will attempt to cross repo blob mount layers from the stack, lifecycle, and buildpacks. If the layers are not mountable the controller will stream them from the source location to the destination location. Because these layers may be very large his operation will put a signfigant load on the kpack controller preventing it from reconciling other builders.

This effectively requires the Buildpack and Stacks to be relocated to the destination of the ClusterBuilder to achieve useable performance within kpack. Currently, this is commonly accomplished by using the kp cli to initialize resources which will automatically relocate to the kp default repository location. This workflow is useable it requires the use of `kp` and makes it difficult to easily initialize kpack components by directly creating resources to images.

**Outcome:**

Allow Buildpacks, Stack, and Lifecycle images to be useable within kpack Builders and ClusterBuilders without needing relocation or degrading the performance of the kpack controller.

**Actions to take:**

Move the creation of Builder and ClusterBuilder images to short-lived pods on the cluster.


**Open Questions:**
* How will the Builder build pods get credentials to the necessary images?


**Prior Art:**

**Risks:**

**Alternatives:**