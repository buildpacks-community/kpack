**Problem:**

Currently, all kpack build pods are created without any mechanism to control or alter the underlying kubernetes node selection. In result, the node selection of the build pods is left to the whim of the kubernetes node scheduler. This is problematic in some deployment topologies where it may be necessary or ideal to restrict which nodes are available for build execution. 

The lack of node customization is also a problem for kpack Windows builds. Currently, when kpack schedules a windows build it attempts to strategically calculate the list of taints that exist across each Windows node in the cluster and set them as tolerations on the build pod. This works successfuly in the simple case but, results in an unschedulable Windows pod when a Windows node is down with a `node.kubernetes.io/unreachable:NoSchedule` taint. Additionally, the kpack build pod logic may inadvertently provide a toleration that results in a windows pod scheduled on an undesriable node. 

**Outcome:**

kpack Image and Build resources should expose a mechanism in the v1alpha2 api to provide nodeSelectors, nodeAffinity, tolerations, schedulerName, and runtimeClassName. These resources should be unmodified from the core v1 Pod api. The contents of these fields should be passed to the underlying build pod without modification except when adding the `kubernetes.io/os` nodeSelector which is currently set by kpack. The only kpack specific validation will be to block Images and Builds that explictily provide the `kubernetes.io/os` nodeSelector.  

On the Image resource these fields should exist under `.spec.build` on the Image spec.

```yaml
apiVersion: kpack.io/v1alpha2
kind: Image
metadata:
  name: sample
spec:
  tag: sample/image
  ...
  
  build:
    runtimeClassName: myclass
    schedulerName: my-scheduler
    tolerations:
    - key: "key1"
      operator: "Exists"
      effect: "NoSchedule"
    nodeSelector:
      disktype: ssd
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: kubernetes.io/e2e-az-name
              operator: In
              values:
              - e2e-az1
              - e2e-az2
```

On the Build resource these fields should exist directly on `.spec` as top level fields on the Build spec.

```yaml
apiVersion: kpack.io/v1alpha2
kind: Build
metadata:
  name: sample
spec:
  tags:
  - sample/image
  ...
  runtimeClassName: myclass
  schedulerName: my-scheduler
  tolerations:
  - key: "key1"
    operator: "Exists"
    effect: "NoSchedule"
  nodeSelector:
    disktype: ssd
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: kubernetes.io/e2e-az-name
            operator: In
            values:
            - e2e-az1
            - e2e-az2
```

With the Image and Build resources allowing users to set node tolerrations, kpack should not attempt to calculate windows node tolerations indepdendently and instead should require windows users to explicitly set the necessary tolerations. 

**Complexity:**

Implementation of this functionality should be of minimal complexity because the contents of the additional fields can be passed directly to the build pod without modification. This functionality will also allow kpack to remove unnecessary complexity in kpack windows toleration selection logic. 

**Prior Art:**

* Tekton allows a [portion of the PodSpec as a PodTemplate](https://tekton.dev/docs/pipelines/podtemplates/) to be set for Tasks and Pipelines which provides support for affinity, nodeSelection, tolerations, schedulerName, and runtimeClassName.

* Argo workflows [support setting tolerations, affinity, nodeSelectors, and a schedulerName](https://argoproj.github.io/argo-workflows/fields/) 

**Alternatives:**

* kpack could provide [node selection fields on the Builder and ClusterBuilder resources](https://github.com/pivotal/kpack/issues/621#issuecomment-892593799) and allow these fields to be applied to all corresponding Image resources. This is something we could explore in a future RFC. Currently, non builder fields are unprecedented on the Builder and ClusterBuilder resources. 

**Risks:**

This will effectively require Windows Image and Build resources to be created in the v1alpha2 api because the tolerations are not accessible in the v1alpha1 api and the current toleration selection logic will no longer be accessible. This is a minor risk because kpack has very few current windows build users.

This will allow end users to provide node selection criteria that results in unscheduleable build pods. 
