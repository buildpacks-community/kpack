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

## Proposal

### Use git2go for Azure Devops Repositories and go-git otherwise

The main reasoning for this is that git2go is the only fully-featured go library that supports the v2 git protocol, but it doesn't support shallow cloning which makes a big difference for builds.

## Actions

### Add required dependencies for libgit2 to the build-init, windows-build-init, and kpack-controller image

Build-init should be the only container that required this as it is the only container using git. This will require changing the build process for build-init, windows-build-init, and kpack-controller (for source resolving).

This will include:
- libgit2
- a c compiler like gcc
- maybe more

### Implement git cloning with git2go

I don't currently see a reason to change our api which is good!

### Add a mechanism for selecting between go-git and git2go implementations

Can this be achieved without users manually selecting this?

### Build build-init, windows-build-init, and kpack-controller with GCO enabled

## Risks

- Drawbacks of using cgo
- Building kpack controller/build-init with required dependencies

## References

- go-git issue for supporting azure devops https://github.com/go-git/go-git/issues/64
- git2go ssh auth support: https://github.com/libgit2/git2go/blob/master/credentials.go#L164
- flux ci/cd implementation of git2go https://github.com/fluxcd/source-controller/tree/d1ee61844d3a51987708bc365d44b3e88cbdcdb9/pkg/git/libgit2
- flux docs for git repository configuration https://toolkit.fluxcd.io/components/source/gitrepositories/
