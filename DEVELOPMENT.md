### Local Development Install

#### Create a cluster

Access to a Kubernetes cluster is needed in order to install the kpack controllers.

An easy way to get a cluster up and running is to use [`kind`](https://kind.sigs.k8s.io/docs/user/quick-start/).

You'll also need:
* [`go`](https://go.dev/doc/install)
* [`pack`](https://buildpacks.io/docs/tools/pack/)
* [`docker`](https://docs.docker.com/get-docker/)
* [`ytt`](https://carvel.dev/ytt/docs/v0.44.0/install/)

#### When using public registries

```bash
kubectl cluster-info # ensure you have access to a cluster
docker login <registry namespace> # must be writable and publicly accessible; e.g., your Docker Hub username or gcr.io/<some-project>
./hack/apply.sh <registry namespace>
```

#### When using private registries

Create a kubernetes secret with the registry creds

```bash
kubectl create secret docker-registry regcreds -n kpack --docker-server=gcr.io/<some-project> --docker-username=_json_key --docker-password="$(cat gcp.json)"
```

Create an overlay to use those registry creds

```bash
cat > ./config/overlay.yaml <<EOF
#@ load("@ytt:overlay", "overlay")

#@overlay/match by=overlay.subset({"kind": "ServiceAccount"}),expects=2
---
#@overlay/match missing_ok=True
imagePullSecrets:
  #@overlay/append
  - name: regcreds
#@overlay/match missing_ok=True
secrets:
  #@overlay/append
  - name: regcreds

#@overlay/match by=overlay.subset({"metadata":{"name":"kpack-controller"}, "kind": "Deployment"})
---
spec:
  template:
    spec:
      containers:
        #@overlay/match by="name"
        - name: controller
          #@overlay/match missing_ok=True
          env:
            #@overlay/append
            - name: CREDENTIAL_PROVIDER_SECRET_PATH
              value: /var/kpack/credentials
          #@overlay/match missing_ok=True
          volumeMounts:
            #@overlay/append
            - name: credentials
              mountPath: /var/kpack/credentials
              readOnly: true
      #@overlay/match missing_ok=True
      volumes:
        #@overlay/append
        - name: credentials
          secret:
            secretName: regcreds
EOF
```

### Running Unit Tests

```bash
make unit
```

### Running End-to-end Tests

* To run the e2e tests, kpack must be installed and running on a cluster
* 🍿 These tests can take anywhere from 20-30 minutes depending on your setup

```bash
IMAGE_REGISTRY=gcr.io/<some-project> \
  IMAGE_REGISTRY_USERNAME=_json_key \
  IMAGE_REGISTRY_PASSWORD=$(cat gcp.json) \
  make e2e
```

* The IMAGE_REGISTRY environment variable must point at a registry with local write access - e.g.

```bash
export IMAGE_REGISTRY="gcr.io/<some-project>"
```

* The KPACK_TEST_NAMESPACE_LABELS environment variable allows you to define additional labels for the test namespace, e.g.

```bash
export KPACK_TEST_NAMESPACE_LABELS="istio-injection=disabled,purpose=test"
```
