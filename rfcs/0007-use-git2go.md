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
[Shallow cloning](https://git-scm.com/docs/git-clone#Documentation/git-clone.txt---depthltdepthgt) basically means we don't pull in the full history which is a performance benefit.

## Proposal

### Use git2go for Azure Devops Repositories and go-git otherwise

The main reasoning for this is that git2go is the only fully-featured go library that supports the v2 git protocol, but it doesn't support shallow cloning which makes a big difference for builds.

## Actions

### Add required dependencies for libgit2 to the build-init image

Build-init should be the only container that required this as it is the only container using git. This will require changing the build process for build-init.

This will include:
- libgit2
- a c compiler like gcc
- maybe more
- Should we spike to see how many dependencies we need, what it would look like to build this, and how would it affect performance?

### Implement git cloning with git2go

This will be in addition to the existing go-git implementation.

### Add a mechanism for selecting between go-git and git2go implementations

Can this be achieved without users manually selecting this?

### Build build-init with GCO enabled

- Should we spike to see how the drawbacks of this?

## Alternatives

### Switch completely to git2go

Lose out on shallow cloning

### Use azure-devops-go-api library

[azure-devops-go-api](https://github.com/microsoft/azure-devops-go-api) is a client wrapper for the azure devops api.

This uses token auth which is a diversion from our current git auth.

This would be a completely different implementation and likely more overhead as it does not support the git-style cloning but instead treats repos as blobs:
https://github.com/microsoft/azure-devops-go-api/blob/dev/azuredevops/git/client.go#L98

## References

- go-git issue for supporting azure devops https://github.com/go-git/go-git/issues/64
- git2go issue for supporting shallow cloning https://github.com/libgit2/git2go/issues/330
- libgit2 issue for supporting shallow cloning https://github.com/libgit2/libgit2/issues/3058
- flux ci/cd implementation of git2go https://github.com/fluxcd/source-controller/tree/d1ee61844d3a51987708bc365d44b3e88cbdcdb9/pkg/git/libgit2
- flux docs for git repository configuration https://toolkit.fluxcd.io/components/source/gitrepositories/
