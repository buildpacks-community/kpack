# Release Process

Kpack release process:
- [Releasing a new Major/Minor Version](#releasing-a-new-majorminor-version)
- [Releasing a new Patch Version](#releasing-a-new-patch-version)
- [Release Automation](#release-automation)


## Releasing a new Major/Minor Version

To produce a major/minor release from `main` a maintainer will:
- Create a new release branch in form `release/<MAJOR-VERSION>.<MINOR-VERSION>.x` from the `main` branch
- Tag release branch as `v<MAYOR-VERSION>.<MINOR-VERSION>.<PATCH-VERSION>` or `v<MAYOR-VERSION>.<MINOR-VERSION>.<PATCH-VERSION>-rc.<NUMBER>` for RC versions
- GitHub Actions will generate a draft GitHub release.
- Publish the [GitHub release][release]:
    - For RCs, the release should be marked as "pre-release"
      >NOTE: A **Release Candidate** (RC) is published and used for further User Acceptance testing (`UAT`). Furthermore, additional RCs may be published based on assessment by the `kpack` maintainers of the **impact**, **effort** and **risk** of including the changes in the upcoming release. Any other changes may be merged into the `main` branch through the normal process, and will make it into the next release.
    - The GitHub release will contain the following:
        - **artifacts**
        - **release notes**
        - **asset checksums**
    - The release notes should be edited and cleaned

## Releasing a new Patch Version

>Patches may be released for backwards-compatible bug fixes and/or dependency bumps to resolve vulnerabilities. There is no patch guarantees for anything other than the latest minor version.

To produce a patch release from an existing `release` branch a maintainer will:
- PR changes into the release branch corresponding to the minor that will be patched.
- Tag release branch as `v<MAYOR-VERSION>.<MINOR-VERSION>.<PATCH-VERSION>` or `v<MAYOR-VERSION>.<MINOR-VERSION>.<PATCH-VERSION>-rc.<NUMBER>` for RC versions
- GitHub Actions will generate a draft GitHub release.
- Publish the [GitHub release][release]:
    - For RCs, the release should be marked as "pre-release"
      >NOTE: A **Release Candidate** (RC) is published and used for further User Acceptance testing (`UAT`). Furthermore, additional RCs may be published based on assessment by the `kpack` maintainers of the **impact**, **effort** and **risk** of including the changes in the upcoming release. Any other changes may be merged into the `main` branch through the normal process, and will make it into the next release.
    - The GitHub release will contain the following:
        - **artifacts**
        - **release notes**
        - **asset checksums**
    - The release notes should be edited and cleaned

## Release automation

- The release candidate process is automated using [GitHub Actions][github-release]. The workflow is triggered by a pushed version tag.
- The release finalization is manual step

[release]: https://github.com/pivotal/kpack/releases
[github-release]: https://github.com/pivotal/kpack/blob/main/.github/workflows/ci.yaml
