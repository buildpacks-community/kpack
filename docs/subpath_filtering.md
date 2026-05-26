# Support for filtering by `subPath`

By default, kpack will check out the whole repository and build the project from
the root. If you set `spec.subPath` to the name of a directory value, kpack will
build that directory only.  When an Image is set to track a branch, and
`subPath` is set, the Image will be rebuilt even if the new commits don't affect
the specified path.  This can lead to unneccessary builds, especially in
monorepos where many Images can track many paths.

The "shallow clone" experimental feature can help in these situations. To enable
the experiment, set the environment variable `GIT_RESOLVER_USE_SHALLOW_CLONE` to
`"true"` in the kpack controller.

When the feature is enabled, builds are skipped entirely when the new revision
does not change anything in the path in question. It does so by tracking the
"tree" of the path in question, and storing this tree value in the
SourceResolver's status.

To compute this "tree" value, kpack will perform a shallow clone of the
reference (only metadata) in-memory, and then the equivalent of a `git ls-tree
<ref> -- path`. This provides the git-calculated hash of the files in the path
and its subdirectories. Any change, even to the file modes, will yield a
different tree value.

When the feature flag is enabled, this method of checking out is used regardless
of the value of `subPath`. In other words, enabling this experiment changes how
kpack resolves all sources, and this can exhert a higher "clone" load on your
git servers.

* If the previous build resulted in a different tree value, a new build will be
  scheduled.
* If the previous build resulted in the same tree value, no new build will be
  scheduled

## Annotated tags

If you are tracking tags, and use annotated tags with subpath filtering and the
`GIT_RESOLVER_USE_SHALLOW_CLONE` feature flag, then the subpath filter will
inhibit new builds, when the subpath is unchanged. If you use this tag metadata
in builds, or expect the tag object sha to be stamped into the image, then you
might want to avoid this combination.
