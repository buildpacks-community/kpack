# Installing kpack

## Prerequisites

1. A Kubernetes cluster version 1.15 or later
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
The Builder image contains buildpacks that will be used to build images with kpack. We recommend starting with the [gcr.io/paketo-buildpacks/builder:base](https://paketo.io/) image which has support for Java, Node and Go.         

```yaml
apiVersion: build.pivotal.io/v1alpha1
kind: ClusterBuilder
metadata:
  name: default
spec:
  image: gcr.io/paketo-buildpacks/builder:base
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
Annotations:  API Version:  build.pivotal.io/v1alpha1
Kind:         ClusterBuilder
Metadata:
  Creation Timestamp:  2020-04-22T15:59:14Z
  Generation:          1
  Resource Version:    1733945
  Self Link:           /apis/build.pivotal.io/v1alpha1/clusterbuilders/default
  UID:                 79ac5b87-9eb0-4e8c-a275-1f20137b911b
Spec:
  Image:          gcr.io/paketo-buildpacks/builder:base
  Update Policy:  polling
Status:
  Builder Metadata:
    Id:       paketo-buildpacks/nodejs
    Version:  v0.0.1
    Id:       paketo-buildpacks/dotnet-core
    Version:  v0.0.1
    Id:       paketo-buildpacks/go
    Version:  v0.0.1
    Id:       paketo-buildpacks/node-engine
    Version:  0.0.178
    Id:       paketo-buildpacks/npm
    Version:  0.1.11
    Id:       paketo-buildpacks/yarn-install
    Version:  0.1.19
    Id:       paketo-buildpacks/dotnet-core-conf
    Version:  0.0.122
    Id:       paketo-buildpacks/dotnet-core-runtime
    Version:  0.0.135
    Id:       paketo-buildpacks/dotnet-core-sdk
    Version:  0.0.133
    Id:       paketo-buildpacks/icu
    Version:  0.0.52
    Id:       paketo-buildpacks/node-engine
    Version:  0.0.178
    Id:       paketo-buildpacks/dotnet-core-aspnet
    Version:  0.0.128
    Id:       paketo-buildpacks/dotnet-core-build
    Version:  0.0.70
    Id:       paketo-buildpacks/dep
    Version:  0.0.109
    Id:       paketo-buildpacks/go-compiler
    Version:  0.0.112
    Id:       paketo-buildpacks/go-mod
    Version:  0.0.96
    Id:       paketo-buildpacks/executable-jar
    Version:  1.2.0
    Id:       paketo-buildpacks/jmx
    Version:  1.1.0
    Id:       paketo-buildpacks/dist-zip
    Version:  1.2.0
    Id:       paketo-buildpacks/google-stackdriver
    Version:  1.1.0
    Id:       paketo-buildpacks/bellsoft-liberica
    Version:  2.3.0
    Id:       paketo-buildpacks/spring-boot
    Version:  1.3.0
    Id:       paketo-buildpacks/encrypt-at-rest
    Version:  1.2.0
    Id:       paketo-buildpacks/build-system
    Version:  1.2.0
    Id:       paketo-buildpacks/debug
    Version:  1.2.0
    Id:       paketo-buildpacks/procfile
    Version:  1.3.0
    Id:       paketo-buildpacks/azure-application-insights
    Version:  1.1.0
    Id:       paketo-buildpacks/apache-tomcat
    Version:  1.1.0
  Conditions:
    Last Transition Time:  2020-04-22T15:59:14Z
    Status:                True
    Type:                  Ready
  Latest Image:            gcr.io/paketo-buildpacks/builder@sha256:fc6c76f22d6d9d2afd654625b63607453cf3ccb65af485905ddfccd812e9eb97
  Observed Generation:     1
  Stack:
    Id:         io.buildpacks.stacks.bionic
    Run Image:  gcr.io/paketo-buildpacks/run@sha256:bfe49e7d1c2c47d980af9dd684047616db872a982dcb2c5515a960d1a962a599
Events:         <none>
```

