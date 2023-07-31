### Local Development Install

#### When using public registries

Access to a Kubernetes cluster is needed in order to install the kpack controllers.

```bash
kubectl cluster-info # ensure you have access to a cluster
./hack/local.sh --help #this will provide all options for building/deploying kpack 
```

#### When using private registries

Create a kubernetes secret with the registry creds

```bash
kubectl create secret docker-registry regcreds -n kpack --docker-server=gcr.io --docker-username=_json_key --docker-password="$(cat gcp.json)"
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
go test ./pkg/...
```

### Running End-to-end Tests
```bash
go test ./test/...
```

* To run the e2e tests, kpack must be installed and running on a cluster

* The KPACK_TEST_NAMESPACE_LABELS environment variable allows you to define additional labels for the test namespace, e.g.

```bash
export KPACK_TEST_NAMESPACE_LABELS="istio-injection=disabled,purpose=test"
```

* The IMAGE_REGISTRY environment variable must point at a registry with local write access 

```bash
IMAGE_REGISTRY=gcr.io/<some-project> go test ./test/...
```
