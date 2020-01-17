# Installing kpack

## Prerequisites

1. A Kubernetes cluster version 1.14 or later
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

1. Create a [ClusterBuilder](builders.md) resource. A ClusterBuilder is a reference to a [Cloud Native Buildpacks builder image](https://buildpacks.io/docs/using-pack/working-with-builders/). 
The Builder image contains buildpacks that will be used to build images with kpack. We recommend starting with the [cloudfoundry/cnb:bionic](https://hub.docker.com/r/cloudfoundry/cnb) image which has support for Java, Node and Go.         

```yaml
apiVersion: build.pivotal.io/v1alpha1
kind: ClusterBuilder
metadata:
  name: default
spec:
  image: cloudfoundry/cnb:bionic
```

Apply the ClusterBuilder yaml to the cluster

```bash
kubectl apply -f cluster-builder.yaml
```

Ensure that kpack has processed the builder by running

```bash
kubectl describe clusterbuilder default
``` 

You should see output similar to the following:

```text
Name:         default
Namespace:    
Labels:       <none>
Annotations:  kubectl.kubernetes.io/last-applied-configuration:
                {"apiVersion":"build.pivotal.io/v1alpha1","kind":"ClusterBuilder","metadata":{"annotations":{},"name":"default"},"spec":{"image":"cloudfou...
API Version:  build.pivotal.io/v1alpha1
Kind:         ClusterBuilder
Metadata:
  Creation Timestamp:  2020-01-17T17:52:19Z
  Generation:          1
  Resource Version:    80893
  Self Link:           /apis/build.pivotal.io/v1alpha1/clusterbuilders/default
  UID:                 1af16b4f-3952-11ea-89bb-025000000001
Spec:
  Image:          cloudfoundry/cnb:bionic
  Update Policy:  polling
Status:
  Builder Metadata:
    Id:       org.cloudfoundry.debug
    Version:  v1.1.17
    Id:       org.cloudfoundry.dotnet-core
    Version:  v0.0.4
    Id:       org.cloudfoundry.go
    Version:  v0.0.2
    Id:       org.cloudfoundry.springautoreconfiguration
    Version:  v1.0.159
    Id:       org.cloudfoundry.buildsystem
    Version:  v1.0.186
    Id:       org.cloudfoundry.procfile
    Version:  v1.0.62
    Id:       org.cloudfoundry.nodejs
    Version:  v1.0.0
    Id:       org.cloudfoundry.distzip
    Version:  v1.0.144
    Id:       org.cloudfoundry.jdbc
    Version:  v1.0.153
    Id:       org.cloudfoundry.azureapplicationinsights
    Version:  v1.0.151
    Id:       org.cloudfoundry.springboot
    Version:  v1.0.157
    Id:       org.cloudfoundry.openjdk
    Version:  v1.0.80
    Id:       org.cloudfoundry.tomcat
    Version:  v1.1.74
    Id:       org.cloudfoundry.googlestackdriver
    Version:  v1.0.96
    Id:       org.cloudfoundry.jmx
    Version:  v1.0.153
    Id:       org.cloudfoundry.archiveexpanding
    Version:  v1.0.102
    Id:       org.cloudfoundry.jvmapplication
    Version:  v1.0.113
    Id:       org.cloudfoundry.dotnet-core-aspnet
    Version:  0.0.97
    Id:       org.cloudfoundry.dotnet-core-build
    Version:  0.0.55
    Id:       org.cloudfoundry.dotnet-core-conf
    Version:  0.0.98
    Id:       org.cloudfoundry.dotnet-core-runtime
    Version:  0.0.106
    Id:       org.cloudfoundry.dotnet-core-sdk
    Version:  0.0.99
    Id:       org.cloudfoundry.icu
    Version:  0.0.25
    Id:       org.cloudfoundry.node-engine
    Version:  0.0.133
    Id:       org.cloudfoundry.dep
    Version:  0.0.64
    Id:       org.cloudfoundry.go-compiler
    Version:  0.0.55
    Id:       org.cloudfoundry.go-mod
    Version:  0.0.58
    Id:       org.cloudfoundry.nodejs-compat
    Version:  0.0.68
    Id:       org.cloudfoundry.npm
    Version:  0.0.83
    Id:       org.cloudfoundry.yarn
    Version:  0.0.94
  Conditions:
    Last Transition Time:  2020-01-17T17:52:19Z
    Status:                True
    Type:                  Ready
  Latest Image:            index.docker.io/cloudfoundry/cnb@sha256:c983fb9602a7fb95b07d35ef432c04ad61ae8458263e7fb4ce62ca10de367c3b
  Observed Generation:     1
  Stack:
    Id:         io.buildpacks.stacks.bionic
    Run Image:  index.docker.io/cloudfoundry/run@sha256:ba9998ae4bb32ab43a7966c537aa1be153092ab0c7536eeef63bcd6336cbd0db
```

