# Legacy Cloud Native Buildpacks Service Bindings

CNB service bindings have been deprecated in `kpack.io/v1alpha2`. They are still available in the `kpack.io/v1alpha1` api but will eventually be removed entirely. Please migrate to the new Kubernetes Service Binding configuration.

## kpack.io/v1alpha1 CNB Service Bindings

kpack image resources can be configured with Service Bindings as described in the [Cloud Native Buildpacks Bindings specification](https://github.com/buildpacks/spec/blob/adbc70f5672e474e984b77921c708e1475e163c1/extensions/bindings.md).

At build-time, service bindings are handled by the buildpacks being used for that Build. Check the desired buildpack documentation for details on the service bindings it supports.

To configure an image resource with a service binding, you must create the following:

* A Secret containing the service binding data
  * The Secret `stringData` field must contain key-value pairs of `<binding file name>:<binding data>`. For each key-value pair, a file will be created that is accessible during build.
* A ConfigMap containing the metadata for the service binding
  * The ConfigMap must have the fields `data.kind` and `data.provider` populated. The buildpacks used to build the OCI image will handle the service bindings based on these fields.
* An Image resource referencing that Secret and ConfigMap in the `spec.build.bindings` field.

### Sample maven app Image resource with a settings.xml service binding:

```yaml
apiVersion: kpack.io/v1alpha1
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
    bindings:
    - name: settings
      secretRef:
        name: settings-xml
   	  metadataRef:
   	    name: settings-binding-metadata
---
apiVersion: v1
kind: Secret
metadata:
  name: settings-xml
type: Opaque
stringData:
  settings.xml: <settings>...</settings>
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings-binding-metadata
data:
  kind: maven
  provider: sample
```

The above example will result in the following files being available during Builds matching the [directory layout](https://github.com/buildpacks/spec/blob/adbc70f5672e474e984b77921c708e1475e163c1/extensions/bindings.md#example-directory-structure) of the CNB spec:

```plain
<platform>
└── bindings
    └── settings
        ├── metadata
        │   ├── kind
        │   └── provider
        └── secret
            └── setting.xml
```
