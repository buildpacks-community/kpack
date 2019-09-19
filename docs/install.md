# Installing kpack

## Prerequisites

1. A Kubernetes cluster version 1.12 or later
1. [kubectl CLI](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
1. Cluster-admin permissions for the current user.
1. Accessible Docker V2 Registry

## Installing-kpack

1. Download the most recent [github release](https://github.com/pivotal/kpack/releases).

   ```bash
   kubectl apply  --filename release.yaml
   ```
1. Ensure that the kpack controller has a status of `Running` using  `kubectl get`.   

   ```bash
   kubectl get pods --namespace kpack --watch
   ```

1. Create a [ClusterBuilder](builders.md) resource. A ClusterBuilder is a reference to a [Cloud Native Buildpacks builder image](https://buildpacks.io/docs/using-pack/working-with-builders/). 
The Builder image contains buildpacks that will be used to build images with kpack. We recommend starting with the [cloudfoundry/cnb:bionic](https://hub.docker.com/r/cloudfoundry/cnb) image which has support for Java, Node and Go.         

```yaml
apiVersion: build.pivotal.io/v1alpha1
kind: ClusterBuilder
metadata:
  name: default-builder
spec:
  image: cloudfoundry/cnb:bionic
```

Apply the ClusterBuilder yaml to the cluster

```bash
kubectl apply -f <name-of-cluster-builder-file.yaml>
```

Ensure that kpack has processed the builder by running

```bash
kubectl get clusterbuilder default-builder -o yaml
``` 

You should see output similar to the following:

```yaml
apiVersion: build.pivotal.io/v1alpha1
kind: ClusterBuilder
metadata:
  creationTimestamp: "2019-09-19T15:05:01Z"
  generation: 1
  name: default-builder
  resourceVersion: "21823241"
  selfLink: /apis/build.pivotal.io/v1alpha1/clusterbuilders/cluster-build-service-builder
  uid: dabd3f65-daee-11e9-827a-42010a800176
spec:
  image: cloudfoundry/cnb:bionic
status:
  builderMetadata:
  - key: org.cloudfoundry.nodejs
    version: 0.0.2-RC3
  - key: org.cloudfoundry.go-compiler
    version: 0.0.24
  - key: org.cloudfoundry.go-mod
    version: 0.0.22
  - key: org.cloudfoundry.dep
    version: 0.0.21
  - key: org.cloudfoundry.openjdk
    version: 1.0.0-RC05
  - key: org.cloudfoundry.buildsystem
    version: 1.0.0-RC05
  - key: org.cloudfoundry.jvmapplication
    version: 1.0.0-RC05
  - key: org.cloudfoundry.azureapplicationinsights
    version: 1.0.0-RC05
  - key: org.cloudfoundry.debug
    version: 1.0.0-RC05
  - key: org.cloudfoundry.googlestackdriver
    version: 1.0.0-RC05
  - key: org.cloudfoundry.jmx
    version: 1.0.0-RC05
  - key: org.cloudfoundry.procfile
    version: 1.0.0-RC05
  - key: org.cloudfoundry.archiveexpanding
    version: 1.0.0-RC05
  - key: org.cloudfoundry.tomcat
    version: 1.0.0-RC05
  - key: org.cloudfoundry.jdbc
    version: 1.0.0-RC05
  - key: org.cloudfoundry.springautoreconfiguration
    version: 1.0.0-RC05
  - key: org.cloudfoundry.springboot
    version: 1.0.0-RC05
  - key: org.cloudfoundry.distzip
    version: 1.0.0-RC05
  - key: org.cloudfoundry.node-engine
    version: 0.0.49
  - key: org.cloudfoundry.npm
    version: 0.0.30
  - key: org.cloudfoundry.yarn
    version: 0.0.28
  conditions:
  - lastTransitionTime: null
    status: "True"
    type: Ready
  latestImage: index.docker.io/cloudfoundry/cnb@sha256:e390f8c7ce696b222197a0e02687aeee6612a9815f78b6f5876de3cb3efd7ba3
  observedGeneration: 1
```

