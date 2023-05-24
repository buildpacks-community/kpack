# Release Process

Kpack release process is composed of 3 phases:
- [Development](#development)
- [Feature Complete](#feature-complete)
- [Release Finalization](#release-finalization)

## Phases

### Development

Our development flow is detailed in [Development](DEVELOPMENT.md).

### Feature Complete

During this period, a **Release Candidate** (RC) is published and used for further User Acceptance testing (`UAT`). Furthermore, additional RCs may be published based on assessment by the `kpack` maintainers of the **impact**, **effort** and **risk** of including the changes in the upcoming release. Any other changes may be merged into the `main` branch through the normal process, and will make it into the next release.

To produce the release candidate a maintainer will:
- Create a new release branch in form `release/<MAYOR-VERSION>.<MINOR-VERSION>.<PATCH-VERSION>-rc.<NUMBER>` yielding a draft GitHub release to be published.
- Publish the [GitHub release][release]:
    - Tag release branch as `v<MAYOR-VERSION>.<MINOR-VERSION>.<PATCH-VERSION>-rc.<NUMBER>`.
    - Release should be marked as "pre-release".
    - The GitHub release will contain the following:
        - **artifacts**
        - **release notes**
    - The release notes should be edited and cleaned

### Release Finalization

The maintainer will:
- Create a new release branch in form `release/<MAYOR-VERSION>.<MINOR-VERSION>.<PATCH-VERSION>` yielding a draft GitHub release to be published.
- Publish the [GitHub release][release] while tagging the release branch as `v<VERSION>`.
    - Tag release branch as `v<MAYOR-VERSION>.<MINOR-VERSION>.<PATCH-VERSION>`.
    - The GitHub release will contain the following:
        - **artifacts**
        - **release notes**

And with that, you're done!

## Release automation

- The release candidate process is automated using [GitHub Actions][github-release]. The workflow is triggered by a push to a release branch.
- The release finalization is manual step

[release]: https://github.com/pivotal/kpack/releases
[github-release]: https://github.com/pivotal/kpack/blob/main/.github/workflows/ci.yaml
