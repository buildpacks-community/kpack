# Installing kpack

## Prerequisites

1. A Kubernetes cluster version 1.18 or later
1. [kubectl CLI](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
1. Cluster-admin permissions for the current user
1. Accessible Docker V2 Registry

## Installing-kpack

1. Download the most recent [github release](https://github.com/pivotal/kpack/releases). The release.yaml is an asset on the release. 

   ```bash
   kubectl apply  --filename release-<version>.yaml
   ```

1. Ensure that the kpack controller & webhook have a status of `Running` using  `kubectl get`.   

   ```bash
   kubectl get pods --namespace kpack --watch
   ```
   
