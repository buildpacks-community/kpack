# Build Service System

Experimental Build Service CRDs.

## Setup

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
2. Create a service account with access to push to the desired docker registry. The example below is for a registry on gcr. Check out [Knative's documentation](https://knative.dev/docs/build/auth/) for more info. 

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: basic-user-pass
  annotations:
    build.knative.dev/docker-0: gcr.io 
type: kubernetes.io/basic-auth
stringData:
  username: <username>
  password: <password>
```

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: service-account
secrets:
  - name: basic-user-pass
```
 
3. Apply an image configuration to the cluster.  

```yaml
apiVersion: build.pivotal.io/v1alpha1
kind: Image
metadata:
  name: sample-image
spec:
  serviceAccount: service-account 
  builder: sample-builder
  image: gcr.io/project-name/app 
  source:
    git:
      url: https://github.com/buildpack/sample-java-app.git
      revision: master
```

4.  See the builds for the image 

```builds
kubectl get cnbbuilds # before the first builds completes you will see a pending status
---------------
NAME                          SHA   SUCCEEDED   REASON
test-image-build-1-ea3e6fa9         Unknown     Pending

```

After a build has completed you will be able to see the built digest

### Running Tests

* To run the e2e tests the build service system must be installed and running on a cluster
* The IMAGE_REGISTRY environment variable must point at a registry with local write access 

```bash
IMAGE_REGISTRY=gcr.io/<some-project> go test ./...
```