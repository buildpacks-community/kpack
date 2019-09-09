# kpack

Experimental Build Service Kubernetes Resource Controllers.  

kpack extends [Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and utilizes kubernetes primitives to provide builds of OCI images as a platform implementation of [Cloud Native Buildpacks](https://buildpacks.io) (CNB).

kpack provides a declarative image type that builds an image and schedules image rebuilds on relevant buildpack and source changes.

kpack also provides a build type to execute a single Cloud Native Buildpack image build.


## Pre requirements

- Kubernetes cluster
- `kubectl` cli
- Docker V2 Registry

## Install

- Use the most recent [github release](https://github.com/pivotal/kpack/releases).

### Creating an Image Resource

1. Create a builder resource. This resource tracks a builder on registry and will rebuild images when the builder has updated buildpacks. 
    ```yaml
    apiVersion: build.pivotal.io/v1alpha1
    kind: Builder
    metadata:
      name: sample-builder
    spec:
      image: cloudfoundry/cnb:bionic
      imagePullSecrets: # optional, if not set builder must be public
      - name: builder-secret
    ```

1. Create a secret for push access to the desired docker registry.
   1. GCR example
      ```yaml
      apiVersion: v1
      kind: Secret
      metadata:
        name: basic-docker-user-pass
        annotations:
          build.pivotal.io/docker: gcr.io
      type: kubernetes.io/basic-auth
      stringData:
        username: <username>
        password: <password>
      ```

    1. Docker Hub example
        ```yaml
        apiVersion: v1
        kind: Secret
        metadata:
          name: basic-docker-user-pass
          annotations:
            build.pivotal.io/docker: index.docker.io
        type: kubernetes.io/basic-auth
        stringData:
          username: <username>
          password: <password>
        ```

1. Create a secret for pull access from the desired git repository. The example below is for a github repository.
    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: basic-git-user-pass
      annotations:
        build.pivotal.io/git: https://github.com
    type: kubernetes.io/basic-auth
    stringData:
      username: <username>
      password: <password>
    ```

1. Create a service account that uses the docker registry secret and the git repository secret.
    ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: service-account
    secrets:
      - name: basic-docker-user-pass
      - name: basic-git-user-pass
    ```

5. Apply an image configuration to the cluster.

    If you would like to build an image from a git repo:
 
    ```yaml
    apiVersion: build.pivotal.io/v1alpha1
    kind: Image
    metadata:
      name: sample-image
    spec:
      tag: gcr.io/project-name/app
      serviceAccount: service-account
      builderRef: sample-builder
      cacheSize: "1.5Gi" # Optional, if not set then the caching feature is disabled
      failedBuildHistoryLimit: 5 # Optional, if not present defaults to 10
      successBuildHistoryLimit: 5 # Optional, if not present defaults to 10
      source:
        git:
          url: https://github.com/buildpack/sample-java-app.git
          revision: master
      build: # Optional
        env:
          - name: BP_JAVA_VERSION
            value: 8.*
        resources:
          limits:
            cpu: 100m
            memory: 1G
          requests:
            cpu: 50m
            memory: 512M
    ```

    If you would like to build an image from an hosted zip or jar:
 
    ```yaml
    apiVersion: build.pivotal.io/v1alpha1
    kind: Image
    metadata:
      name: sample-image
    spec:
      tag: gcr.io/project-name/app
      serviceAccount: service-account
      builderRef: sample-builder
      cacheSize: "1.5Gi" # Optional, if not set then the caching feature is disabled
      failedBuildHistoryLimit: 5 # Optional, if not present defaults to 10
      successBuildHistoryLimit: 5 # Optional, if not present defaults to 10
      source:
        blob:
          url: https://storage.googleapis.com/build-service/sample-apps/spring-petclinic-2.1.0.BUILD-SNAPSHOT.jar
      build: # Optional
        env:
          - name: BP_JAVA_VERSION
            value: 8.*
        resources:
          limits:
            cpu: 100m
            memory: 1G
          requests:
            cpu: 50m
            memory: 512M
    ```

6.  See the builds for the image

    ```builds
    kubectl get cnbbuilds # before the first builds completes you will see a unknown (building) status
    ---------------
    NAME                          SHA   SUCCEEDED
    sample-image-build-1-ea3e6fa9         Unknown  

    ```

After a build has completed you will be able to see the built digest

### Tailing Logs from Builds

Use the log tailing utility in `cmd/logs`

```bash
go build ./cmd/logs
```

The logs tool allows you to view the logs for all builds for an image: 

```bash
./logs -image <IMAGE-NAME>
```

To view logs from a specific build use the build flag:  

```bash
./logs -image <IMAGE-NAME> -build <BUILD-NUMBER>
```

## Local Development Install

Access to a Kubernetes cluster is needed in order to install the kpack controllers.

```bash
kubectl cluster-info # ensure you have access to a cluster
./hack/apply.sh <IMAGE/NAME> # <IMAGE/NAME> is a writable and publicly accessible location 
```

If generation of code is needed you should run the following commands:
```bash
go mod vendor # Need a location to load the code generator tool from
./hack/update-codegen.sh
```

### Running Tests

* To run the e2e tests, kpack must be installed and running on a cluster
* The IMAGE_REGISTRY environment variable must point at a registry with local write access 

    ```bash
    IMAGE_REGISTRY=gcr.io/<some-project> go test ./...
    ```
