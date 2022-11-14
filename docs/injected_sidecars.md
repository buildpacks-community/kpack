# Support for Injected Sidecars

In environments that require sidecars to be running in pods to allow for outbound network traffic (i.e Istio), kpack can
optionally run builds in standard containers instead of init containers. This will cause the build pod to wait for all
sidecars to be running before it attempts to run any build steps. To enable this feature, set the environment
variable `INJECTED_SIDECAR_SUPPORT` to `"true"` on the kpack controller. For more info, take a look at
the [RFC](../rfcs/0010-support-injected-sidecars.md).

### A Note on Resource Requests/Limits

The kpack Image and Build resources allow for the configuration of a build
[pod's resource request/limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)
. This field is passed into the pod from the fields on the Image/Build resources and added to every build container
without modification. When calculating the pod's overall request/limit, Kubernetes will treat init containers and
regular containers differently due to the fact that init containers are guaranteed to be the only container in the pod
running at any given time. In an init container build, The request used when scheduling the pod on the node is the max
request from any container, which will end up being equal to the request passed in. "Standard"
containers, on the other hand, will all be running at the same time, even if they are just waiting to run the next step
in a build, so Kubernetes will take the sum of all resource requests on each container and use that value when
scheduling. This will result in a 7x larger request than what is passed in from the Image/Build resources. When using
the resources field with the `INJECTED_SIDECAR_SUPPORT=true` you will need to pass a 7x smaller request value to
maintain the same scheduling behavior when compared to init container builds.
When [setting the limit value](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#how-pods-with-resource-limits-are-run)
, you should not use a smaller value because it can cause your build to be terminated if one of the containers exceeds
the provided limit. 