**Problem:**
The upstream buildpacks has [removed support for windows](https://github.com/buildpacks/rfcs/blob/main/text/0133-remove-windows-containers-support.md), which will lead to the removal of a windows lifecycle. Without this, kpack cannot run windows builds.

**Outcome:**
Windows support will be removed from kpack. This feature is not used and [discussions to remove it](https://github.com/buildpacks-community/kpack/discussions/1366) were ongoing before the removal of Windows support in the lifecycle.

**Actions to take:**
Remove all windows support from kpack. Error if an existing Windows builder is used during a build.

**Complexity:**
Low Complexity

**Risks:**
Due to the fact that there are no known users of Windows support in kpack, there is very little risk with this proposal
