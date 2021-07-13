# Bumping the build API version

As an example, we show how to update from `v1alpha1` to `v1alpha2`

1. Copy `./pkg/api/build/v1alpha1` to `./pkg/api/build/v1alpha2`
1. Set package to `v1alpha2` in `./pkg/api/build/v1alpha2`
1. Update `./pkg/api/build/v1alpha2/register.go` to include

```go
var SchemeGroupVersion = schema.GroupVersion{Group: build.GroupName, Version: "v1alpha2"}
```

1. Remove now outdated methods and tests from `v1alpha1`
1. Search and replace:
```
buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha1" -> buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
```
1. Update `./hack/update-codegen.sh`
    Append `v1alpha2` to group: `deepcopy,client,informer,lister`
    ```bash
    bash "${CODEGEN_PKG}"/generate-groups.sh "deepcopy,client,informer,lister" \
    github.com/pivotal/kpack/pkg/client github.com/pivotal/kpack/pkg/apis \
    "build:v1alpha1,v1alpha2" \
    --output-base "${TMP_DIR}/src" \
    --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate/boilerplate.go.txt
    ```
1. Update `./hack/openapi-codegen.sh` to include `v1alpha2`
```bash
${OPENAPI_GEN_BIN} \
  -h ./hack/boilerplate/boilerplate.go.txt \
  -i github.com/pivotal/kpack/pkg/apis/build/v1alpha2,github.com/pivotal/kpack/pkg/apis/core/v1alpha1 \
  -p ./pkg/openapi \
  -o ./

# VolatileTime has custom json encoding/decoding that does not map to a proper json schema. Use a basic string instead.
sed -i.old 's/Ref\:         ref(\"github.com\/pivotal\/kpack\/pkg\/apis\/core\/v1alpha2.VolatileTime\"),/Type: []string{\"string\"}, Format: \"\",/g' pkg/openapi/openapi_generated.go
```
1. Run `./hack/openapi-codegen.sh` and `./hack/update-codegen.sh`
1. Replace all occurrences of `Kpack().V1alpha1()` with `Kpack().V1alpha2()` (!*NOT* in `generic.go`)
1. Replace all occurrences of `KpackV1alpha2()` with `KpackV1alpha2()` (!*NOT* in `pkg/client`)
1. Adapt `config/*.yaml`
  * `v1alpha2`
  * `served`
  * `storage`
