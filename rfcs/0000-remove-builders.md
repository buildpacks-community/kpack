*DRAFT* 
Today, kpack has a complicated hierarchy of builder resources (Builder & ClusterBuilder & CustomBuilder & CustomClusterBuilder). Navigating this hierarchy is hard to explain and has consistently been confusing in early feedback. This is complicated by the recommended kpack experience being "CustomBuilders" when it appears that the obvious default is "Builder" or "ClusterBuilder".

Additionally, the "Builder's"  limited feature set makes it difficult for users to actualize and understand kpack's functionality making it a poor onboarding experience for new kpack community members and discouraging users from utilizing the Builder resource. Whereas onboarding with the explicit Stack, Store, Builder resources should make it easier to understand kpack's re-build logic. If we removed the existing Builder resource to invest in simplified CustomBuilder tooling and documentation we could focus on providing the best unified experience for users and remove the toil that Builders create.    

**Outcomes:**
- Reduce kpack onboarding confusion resulting from the need to choose the right builder kind
- Encourage kpack users to utilize CustomBuilders which provide the best kpack experience.
- Simplify a future kpack cli from needing to use the terms `custom-cluster-builder` and `cluster-builder`.
- Reduce the maintenance overhead of maintaining both sets of Builders 

**Actions to take:**
* We rename CustomBuilder to Builder and ClusterCustomBuilder to ClusterBuilder and remove the Builder/ClusterBuilder resource.
* We provide a kpack cli that initializes a set of ~~(Custom)~~Builders, Stacks, Stores to make onboarding as simple as the previous builder experience.
* We consider recreating Builders in an eventual kpack monitors project.    

**Complexity:**
N/A  

**Issues/stories/prior art:**
We have already started 

**Alternatives:**
* Rename Builder to something like pre-built Builder.  

**Risks:**
We will need to invest in tutorials and documentation to make onboarding on these resources as simple as the current kpack onboarding experience. 
