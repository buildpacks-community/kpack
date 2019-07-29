# Build Service System

Experimental Build Service CRDs.

## Pre requirements

- Kubernetes cluster

## Install

- Use the most recent github release.

## Local Development Install

Access to a Kubernetes cluster is needed in order to install the build service system controllers.

```bash
kubectl cluster-info # ensure you have access to a cluster
./hack/apply.sh <IMAGE/NAME> # <IMAGE/NAME> is a writable and publicly accessible location 
```

### Creating an Image Resource

1. Create a builder resource. This resource tracks a builder on registry and will rebuild images when the builder has updated buildpacks. 
    ```yaml
    apiVersion: build.pivotal.io/v1alpha1
    kind: Builder
    metadata:
      name: sample-builder
    spec:
      image: cloudfoundry/cnb:bionic
    ```

2. Create a secret for push access to the desired docker registry. The example below is for a registry on gcr.
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

3. Create a secret for pull access from the desired git repository. The example below is for a github repository.
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

4. Create a service account that uses the docker registry secret and the git repository secret.
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
    ```yaml
    apiVersion: build.pivotal.io/v1alpha1
    kind: Image
    metadata:
      name: sample-image
    spec:
      serviceAccount: service-account
      builderRef: sample-builder
      image: gcr.io/project-name/app
      cacheSize: "1.5Gi"
      failedBuildHistoryLimit: 5
      successBuildHistoryLimit: 5
      source:
        git:
          url: https://github.com/buildpack/sample-java-app.git
          revision: master
    ```

6.  See the builds for the image

    ```builds
    kubectl get cnbbuilds # before the first builds completes you will see a pending status
    ---------------
    NAME                          SHA   SUCCEEDED   REASON
    test-image-build-1-ea3e6fa9         Unknown     Pending

    ```

After a build has completed you will be able to see the built digest

### Tailing Logs from Builds

Use the log tailing utility in `cmd/logs`

```bash
go build ./cmd/logs
```

The logs tool allows you to view the logs for all builds for an image: 

```bash
logs  -kubeconfig <PATH-TO-KUBECONFIG> -image <IMAGE-NAME>
```

To view logs from a specific build use the build flag:  

```bash
logs  -kubeconfig <PATH-TO-KUBECONFIG> -image <IMAGE-NAME> -build <BUILD-NUMBER>
```

### Running Tests

* To run the e2e tests the build service system must be installed and running on a cluster
* The IMAGE_REGISTRY environment variable must point at a registry with local write access 

    ```bash
    IMAGE_REGISTRY=gcr.io/<some-project> go test ./...
    ```