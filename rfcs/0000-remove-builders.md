Today, kpack has a complicated hierarchy of builder resources (Builder & ClusterBuilder & CustomBuilder & CustomClusterBuilder). Navigating this hierarchy is hard to explain and has consistently been confusing in early feedback. This is complicated by the recommended kpack experience being "CustomBuilders" when it appears that the obvious default is "Builder" or "ClusterBuilder".

The existing "Builder" relies on a polling mechanism to monitor an external Builder for buildpack and stack updates. External polling mechanisms in core kpack types have been a source of confusion that contradicts the pure declarative philosophy outlined in the [kpack monitors rfc](https://github.com/pivotal/kpack/pull/433).

The "Builder's" limited control makes it difficult for users to actualize and understand kpack's functionality and make it a poor onboarding experience for new kpack community members. This discourages users from utilizing the extensible (Custom)Builder resource. Whereas onboarding with the explicit Stack, Store, Builder resources should make it easier to understand kpack's re-build logic. If we removed the existing Builder resource to invest in simplified CustomBuilder tooling and documentation we could focus on providing the best-unified experience for users and remove the toil that Builders create.

Finally, for users to get the most value out of kpack they will eventually need to utilize the (Custom)Builder toolchain to leverage the granular control of buildpacks and stack images. For users to do that the cloud-native buildpacks community will need to have a robust ecosystem of buildpackages and stack images. We can encourage this community migration by discouraging or removing our existing Builder support.

## Outcomes:
- Reduce kpack onboarding confusion that results from the need to choose the right builder kind.
- Encourage kpack users to utilize CustomBuilders which will provide the best kpack onboarding experience.
- Simplify a future kpack cli from needing to use the terms `custom-cluster-builder` and `cluster-builder`.
- Reduce the maintenance overhead of maintaining both sets of Builder resources. 

## Actions to take:
* Rename the CustomBuilder to Builder and ClusterCustomBuilder to ClusterBuilder and remove the Builder/ClusterBuilder resource.
* Provide a kpack cli with functionality that initializes a set of ~~(Custom)~~Builders, Stacks, Stores to make onboarding and setup as simple as the previous builder experience.
* We will consider recreating Builders in the eventual kpack monitors project.   

## Complexity:
There is already [active](https://github.com/pivotal/kpack/pull/437) [PRs](https://github.com/pivotal/kpack/pull/434) with major API renames. This is an ideal release window to include another major breaking api change.     

 ## Prior Art:
N/A 

## Alternatives:

The Builder could be renamed to something like "PreBuilt" Builder. This will allow users to continue utilizing external Builders. However, this approach does not resolve the underlying problems with the Builder concept and may continue to encourage users to use the PreBuiltBuilder.


## Risks:
We will need to invest in creating tutorials and documentation to make the onboarding of these resources as simple as the current kpack onboarding experience. 

We will need to help facilitate migrations for any user or platform that is using the existing Builder.
