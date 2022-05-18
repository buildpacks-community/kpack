### Local Development Install

Access to a Kubernetes cluster is needed in order to install the kpack controllers.

```bash
kubectl cluster-info # ensure you have access to a cluster
./hack/apply.sh <IMAGE/NAME> # <IMAGE/NAME> is a writable and publicly accessible location 
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

### Libgit2 v1.3.0 Dependency and Installation

Several unit tests depend upon libgit2 v1.3.0.

__macOS Installation Instructions (Intel):__

1. Install `cmake` and `pkg-config`
```bash
brew install cmake pkg-config
```
2. Verify no conflicting version of `libgit2` is installed

```bash
pkg-config --print-provides libgit2
```

You should expect output which reads either: `Package libgit2 was not found in the pkg-config search path` or `libgit2 = 1.3.0`. If a version of `libgit2` other than `1.3.0` is reported as installed, consider uninstalling `1.3.0` or running the unit tests in a different environment.

3. Download `libgit2 v1.3.0` source code
```bash
mkdir libgit2-install && cd libgit2-install
curl -L -O https://github.com/libgit2/libgit2/archive/refs/tags/v1.3.0.tar.gz
```

4. Compile and Install
```bash
tar -zxf libgit2-1.3.0.tar.gz
cd libgit2-1.3.0
mkdir build && cd build
cmake .. -DCMAKE_INSTALL_PREFIX=/usr/local -DCMAKE_OSX_ARCHITECTURES="x86_64"
cmake --build . --target install
```

5. Verify installation
```bash
pkg-config --print-provides libgit2
```

You should expect output which reads: `libgit2 = 1.3.0`
