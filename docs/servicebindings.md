# Service Bindings

kpack Images can be configured with Service Bindings as described in the [Kubernetes Service Bindings specification](https://github.com/k8s-service-bindings/spec).

At build-time, service bindings are handled by the buildpacks being used for that Image. Check the desired buildpack homepage for documentation on the service bindings it supports. Buildpack homepages can be found with `kp clusterstore status <store-name>`.

There are two ways to configure service bindings:

1. Natively using a Secret (most common use case for kpack users)
2. Using ProvisionedServices as described in the [specification](https://github.com/k8s-service-bindings/spec#provisioned-service) (requires additional operator configuration external to kpack)

## Create a Service Binding with a Secret

Requirements:

* A Secret containing the service binding data
  * The Secret `stringData` field **must** contain a key-value pairs of `type:<binding type>`. The buildpacks will use read this type
  * The Secret `type` (not `stringData.type`) is **recommended** to be set to `service.binding/<binding type>` where `<binding type>` is the value of the key set in the above bullet.
  * The Secret `stringData` field may contain any additional key-value pairs of `<binding file name>:<binding data>`. For each key-value pair, a file will be created that is accessible during build.
* An Image in the same namespace referencing that Secret in the `spec.build.services` field as an [ObjectReference](https://www.k8sref.io/docs/common-definitions/objectreference-/).

### Sample maven app Image with a settings.xml service binding:

```yaml
apiVersion: kpack.io/v1alpha2
kind: Image
metadata:
  name: sample-binding-with-secret
spec:
  tag: my-registry.com/repo
  builder:
    kind: ClusterBuilder
    name: default
  source:
    git:
      url: https://github.com/buildpack/sample-java-app.git
      revision: 0eccc6c2f01d9f055087ebbf03526ed0623e014a
  build:
    services:
    - name: settings-xml
      kind: Secret
      apiVersion: v1
---
apiVersion: v1
kind: Secret
metadata:
  name: settings-xml
type: service.binding/maven
stringData:
  type: maven
  provider: sample
  settings.xml: <settings>...</settings>
```

The above example will result in the following files being available during Builds of the Image matching the [directory layout](https://github.com/k8s-service-bindings/spec#application-projection) of the K8s Service Binding Application Projection spec:

```plain
$SERVICE_BINDING_ROOT
└── account-database
    ├── type
    ├── provider
    └── settings.xml
```

`$SERVICE_BINDING_ROOT` will be set to `<platform>/bindings`

## Create a Service Binding with a ProvisionedService

kpack is fully compliant with the Kubernetes Service Binding Spec and supports bindings with [ProvisionedServices](https://github.com/k8s-service-bindings/spec#provisioned-service).

ProvisionedServices are custom resources that implement the provisioned service duck type and must be configured external to kpack. ProvisionedServices provide the name of a Secret in `status.binding.name`. For more details, see the [Kubernetes Service Bindings specification](https://github.com/k8s-service-bindings/spec).

Requirements for a service bindings using ProvisionedService:

* A resource that implements the ProvisionedService duck type and
* A Secret containing the service binding data
  * The Secret `stringData` field **must** contain a key-value pairs of `type:<binding type>`. The buildpacks will use read this type
  * The Secret `type` (not `stringData.type`) **must** be set to `service.binding/<binding type>` where `<binding type>` is the value of the key set in the above bullet.
  * The Secret `stringData` field may contain any additional key-value pairs of `<binding file name>:<binding data>`. For each key-value pair, a file will be created that is accessible during build.
* An Image in the same namespace referencing the ProvisionedService in the `spec.build.services` field as an [ObjectReference](https://www.k8sref.io/docs/common-definitions/objectreference-/).

### Sample maven app Image with a settings.xml service binding using a ProvisionedService:

```yaml
---
apiVersion: kpack.io/v1alpha2
kind: Image
metadata:
  name: sample-binding-with-secret
spec:
  tag: my-registry.com/repo
  builder:
    kind: ClusterBuilder
    name: default
  source:
    git:
      url: https://github.com/buildpack/sample-java-app.git
      revision: 0eccc6c2f01d9f055087ebbf03526ed0623e014a
  build:
    services:
    - name: sample-ps
      kind: CustomProvisionedService
      apiVersion: v1
---
apiVersion: v1alpha1
kind: CustomProvisionedService
metadata:
  name: sample-ps
...
status:
  binding:
    name: settings-xml
---
apiVersion: v1
kind: Secret
metadata:
  name: settings-xml
type: service.binding/maven
stringData:
  type: maven
  provider: sample
  settings.xml: <settings>...</settings>
```
