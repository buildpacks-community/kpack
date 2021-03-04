## Problem:
Git repositories with Azure Devops does not work with kpack.

The issue stems from using the go-git library which does not support the git v2 protocol which is required with Azure Devops repos:
- https://github.com/go-git/go-git/issues/64

## Outcome
Users can use Azure Devops for git repositories

## Background

Azure Devops represents a significant use case

[go-git](https://github.com/go-git/go-git) is the git implementation library in pure go

[git2go](https://github.com/libgit2/git2go) is a library for go bindings to the git method library in c, [libgit2](https://libgit2.org/).

## Options

### 1. Use git2go

The main reasoning for this is that git2go is the only fully-featured go library that supports the v2 git protocol which is required for Azure Devops git.

#### Actions

##### Add required dependencies for libgit2 to the build-init, windows-build-init, and kpack-controller image

Build-init should be the only container that required this as it is the only container using git. This will require changing the build process for build-init, windows-build-init, and kpack-controller (for source resolving).

This will include:
- Bionic:
    - [libgit2-26](https://packages.ubuntu.com/bionic/libgit2-26)
    - [cmake](https://packages.ubuntu.com/bionic/cmake)
    - [pkg-config](https://packages.ubuntu.com/bionic/pkg-config)
- Windows: not sure

Note:
- Only libgit2-26 is available on ubuntu bionic which is incompatible with the master branch of git2go

##### Implement git cloning with git2go

##### Implement git source monitoring (kpack controller) with git2go

##### Build build-init, windows-build-init, and kpack-controller with GCO enabled

- We may want to statically build git2go to guarantee api compatibility with libgit2
  - Maybe we don't have to since we only support bionic?

## Risks

- Drawbacks of using cgo
- Developing with this new library is already giving us issues
- Building kpack controller/(windows-)build-init with new required dependencies

### 2. Use git binary to fetch and go-git to monitor git source

The git binary would work with azure devops!

#### Actions

##### Add required dependencies for git to the build-init, windows-build-init, and kpack-controller image

Build-init should be the only container that required this as it is the only container using git. This will require changing the build process for build-init, windows-build-init, and kpack-controller (for source resolving).

This will include:
- [bionic](https://packages.ubuntu.com/bionic/git)
- windows (not sure)

Note:

##### Implement git cloning by shelling out to git binary

## Risks

- Building (windows-)build-init with new required dependencies
- Can't monitor git source for azure devops (must update revisions manually)

## References

- go-git issue for supporting azure devops https://github.com/go-git/go-git/issues/64
- git2go ssh auth support: https://github.com/libgit2/git2go/blob/master/credentials.go#L164
- flux ci/cd implementation of git2go https://github.com/fluxcd/source-controller/tree/d1ee61844d3a51987708bc365d44b3e88cbdcdb9/pkg/git/libgit2
- flux docs for git repository configuration https://toolkit.fluxcd.io/components/source/gitrepositories/
