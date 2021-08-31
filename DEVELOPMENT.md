## Setup

### Prerequisites

  * [libgit2](https://libgit2.org/) >= 1.1.0
    * macOS: `brew install libgit2`

## Unit Tests

#### Run

```
make unit
```

## Local Registry

#### Prerequisites

  * [docker](https://docs.docker.com/get-docker/)
  * [ngrok](https://ngrok.com/) - or any public routing solution

1. In one terminal, start a docker registry
    ```shell
    docker run -p 5000:5000 --rm -e REGISTRY_STORAGE_DELETE_ENABLED=true registry:2
    ```
2. In another terminal, route port 5000:
    ```shell
    ngrok http 5000
    ```

You can now use image names with the ngrok url. For example,

```
<random>.ngrok.io/myorg/myapp
```

and use `IMAGE_REGISTRY=<random>.ngrok.io/myorg` environment variable.

## Local Install

#### Prerequisites

  * [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
    * macOS: `brew install kubectl`
  * [pack](https://buildpacks.io/docs/tools/pack/)
    * macOS: `brew install pack`

#### Install

Access to a Kubernetes cluster is needed in order to install the `kpack` controllers.

```shell
# ensure you have access to a cluster
kubectl cluster-info

# 
./hack/apply.sh <IMAGE_REGISTRY>
```

_NOTE: <IMAGE_REGISTRY> must be a writable and publicly accessible. You may want to use a [local registry](#local-registry)._


## E2E Tests

#### Requirements

  * `kpack` must be [installed](#local-install) and running on a cluster.
  * A publically available image registry must be used.
    * You may use something like [gcr.io](https://cloud.google.com/container-registry/) or a [local registry](#local-registry) (routed publically).

#### Configuration

|Environment Variable | Description | Example Values
|---                  |---          |---
| `KPACK_TEST_NAMESPACE_LABELS` | Define additional labels for the test namespace | `istio-injection=disabled,purpose=test`
| `IMAGE_REGISTRY` | Image Registry to use when creating images. | `gcr.io/<project>`

#### Run

```
make e2e
```