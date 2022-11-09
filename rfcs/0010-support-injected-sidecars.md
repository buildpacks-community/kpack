## Problem

Currently, kpack executes each stage of the [CNB Lifecycle][cnb-lifecycle] inside of
individual [init containers][init-containers]. This has two major benefits:

1. Different stages of the lifecycle are isolated from one another allowing the credentials required to push to the
   registry to be only mounted in the containers that need them, preventing a rogue [BuildPack][buildpack] from reading
   the secrets.

2. We can very easily define the order of the steps occurring as well as waiting for the previous step to complete
   before continuing on. Additionally, failures in one init container cause the entire build to fail without additional
   logic on our part.

As kpack has grown, we have seen a need to support [service meshes][istio-service-mesh] such as [Istio][istio], which do
not work when you have init containers that need to reach out to the network. This is
a [well documented][istio-init-container-compatability] technical limitation related to
Istio's [automatic sidecar injection][istio-sidecar-injection]. The limitation is due to Istio adding a sidecar
container to the kpack build pod which runs [envoy proxy][envoy-proxy] and sets up traffic from the build containers to
be routed through the proxy. Due to the fact that init containers run before any normal container can start, kpack
builds are unable to reach the network because they are attempting to communicate through the envoy proxy container that
has not and will not start until the build completes.

To get around this issue, kpack currently applies an annotation (`sidecar.istio.io/inject=false`) to all build pods to
disable Istio sidecar injection. This presents a problem where users might want to take advantage of the various
features of Istio, but cannot because kpack is bypassing it.

[buildpack]: https://buildpacks.io/

[cnb-lifecycle]: https://buildpacks.io/docs/concepts/components/lifecycle/

[envoy-proxy]: https://www.envoyproxy.io/

[init-containers]: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/

[istio]: https://istio.io/

[istio-sidecar-injection]: https://istio.io/latest/docs/setup/additional-setup/sidecar-injection/#injection

[istio-service-mesh]: https://istio.io/latest/about/service-mesh/#what-is-a-service-mesh

[istio-init-container-compatability]: https://istio.io/latest/docs/setup/additional-setup/cni/#compatibility-with-application-init-containers

## Outcome

When kpack is running with `SUPPORT_INJECTED_SIDECARS=true`, builds can execute successfully in environments where
sidecars are executed into the build pods (i.e. istio proxy sidecar injection).

## Actions to take

1. Create a new binary that can be used as a new entrypoint for build steps. The purpose of this binary will be to allow
   for specific build steps to depend on other build steps to complete before executing since the containers will be
   running concurrently. There will also need to be a mechanism for propagating errors in one build step so that a build
   can exit on error.

   Example of possible implementation is in the [spike][kpack-spike-build-waiter].
2. Convert the existing init containers in to be regular containers and add modify the entrypoint to be the new binary.
3. We will put this new behavior behind a feature gate (Environment variable added to kpack controller deployment
   called `SUPPORT_INJECTED_SIDECARS`) and only execute in regular containers if enabled by the user. In
   the event that the feature flag is not set or set to false, kpack will continue to execute builds in init containers.
4. Due to the fact that sidecars such as istio-proxy, do not exit when all other containers exit, we will need to add a
   method for stopping the proxy once a build exits
    1. To do this we can update the running pod's `spec.activeDeadlineSeconds` to 1 which will cause the pod to go into
       a failed state and kill all containers after 1 second.
    2. This will kill the sidecar, but it will also leave our pod in a failed state with the message `DeadlineExceeded`,
       leading the reconciler to believe the build has failed. To circumvent this we will need allow pods to
       fail with the reason `DeadlineExceeded` as long the completion container has successfully completed.
5. We will also need a way to delay the start of a build until the sidecars are ready in the event that they are
   required for network communication  (i.e istio)
    1. To do this we will have the prepare step wait on a file passed in through
       the [kubernetes downward api][downward-api]. This file can be provided from a `build ready` annotation that is
       added to the pod when by the build reconciler when all the containers in the pod are in a ready state.

[kpack-spike-build-waiter]: https://github.com/pivotal/kpack/blob/execute-builds-in-regular-containers/cmd/build-waiter/main.go

[step-modifier]: https://github.com/pivotal/kpack/blob/44257a70c2c9e9ac5703eadc871c7c9be22cfadc/pkg/apis/build/v1alpha2/build_pod.go#L181

[downward-api]: https://kubernetes.io/docs/concepts/workloads/pods/downward-api/

## Complexity

* Our current build logging relies heavily on the fact that our builds exist in init containers so that logic will
  need to be reworked to support this change.
* We would need to create an implementation specific to windows builds as well if windows support is needed

## Prior Art

### 1. tekton

https://github.com/tektoncd/pipeline/tree/main/cmd/entrypoint#waiting-for-sidecars

### 2. kpack spike

https://github.com/pivotal/kpack/pull/1019

## Alternatives

* We can create tekton [`TaskRuns`][tekton-task-run] to execute each step of the build since tekton has come up with
  their own way to solve this problem which
  involves [replacing the sidecar container images with a "noop" image][tekton-stop-sidecar].
    * Note: Tekton currently will only stop sidecars (including the istio proxy) if at least one sidecar is defined in
      the `TaskSpec`.
* We can build the Tekton entrypoint code as an image to use in kpack.
* We could execute all build steps in one standard container, similar to [Pack's][pack-build-docs] `pack build` command
  with the `--trust-builder` flag.
    * This would involve us likely extending the `build-init` binary functionality to execute the lifecycle steps after
      downloading the source.
    * We would also have to modify how we build builder images to include this binary as well as the lifecycle

[pack-build-docs]: https://buildpacks.io/docs/tools/pack/cli/pack_build/

[tekton-task-run]: https://tekton.dev/docs/pipelines/taskruns/#overview

[tekton-stop-sidecar]: https://github.com/tektoncd/pipeline/blob/210dbe8965ab5fa9c0ca53e164b86d441899b763/pkg/reconciler/taskrun/taskrun.go#L240-L283

## Risks

* Because all the containers will be running at once, the pod will request more resources than it does today as due to
  the [resource usage calculations][pod-resources].
    * Potential Mitigation: if a build spec specifies a resource limit, we can divide it by the number of containers so
      that the pod's request is the same as it was with init containers
* There is a lot of additional logic required to order standard containers while kubernetes offers a primitive to
  support ordered container execution by using init containers.

[pod-resources]: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#resources
