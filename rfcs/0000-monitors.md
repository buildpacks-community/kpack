Today, kpack provides resources to declaratively model the desired state for cnb built images and builders. These resources are simple to understand and deterministic which make them good building blocks for larger platform and CI/CD implementations.  

However, kpack also provides some resources with polling mechanisms that continually check for new updates (Git and Builders). These resources conflate the declarative configuration with a possible update strategy. 

This has resulted in confusion. Many users don’t desire a kpack polling mechanism and we have needed to create mechanisms to explicitly turn it off (PollingStrategy). Other users have been confused because they don’t want to utilize polling but assume it is required. This is especially common with git source monitoring. Explaining and documenting the role of these resources has also been made more difficult by their dual responsibilities. 

Despite these complexities, we still have had feature requests to provide more polling mechanisms in kpack. The Spring Team would like to be able to monitor a maven repo for updates. Other language ecosystems will likely have similar requests. At the same time, we suspect community kpack users would like to be able to leverage updates to Stacks and Stores without setting up external pipelines. 

### Goals:
- Allow users to leverage polling of external resources without violating the sanctity of the single responsibility principle.
- Experiment with monitoring and polling use-cases without significant API churn on kpack core resources.
- Allow kpack “core” to align with the traditional expectations for kubernetes resources.
- Allow kpack users to leverage Stack & Store updates without setting up external pipelines. 
- Support niche use-cases without introducing language-specific idioms in core kpack resources such as maven coordinates.
- Decouple kpack critical core functionality from possibly unstable i/o intensive polling resources.

### Actions to take:
Start a new project/repo called kpack-monitors which will provide a set of custom resources that monitor external systems for relevant updates to kpack resources. 

*The immediate candidates for new resources are*:
_MonitorStore_: A resource that will monitor a registry for new-build packages at a configured tag. 
_MonitorStack_: A resource that will monitor stack images for updated digests.
_MonitorMavenSource_: A resource that will monitor a maven repo for new versions of a published jar artifact. 

*Possible Candidates*:
_MonitorGitSource_: A resource that monitors a git repo for updated revisions.

### Prior Art:
Extracting independent focused projects for related resources with distinct lifecycles is a common pattern in the Kubernetes ecosystem. 

Additionally, there are a couple of add-on projects which provide resources to handle updates from external sources and are good corollaries to kpack-monitors: 

The tekton pipeline has a sibling [tekton triggers](https://github.com/tektoncd/triggers) project which provides webhook trigger resources to initiate tekton pipelines. 

The argo project contains [argo events](https://argoproj.github.io/argo-events/) which provides tooling to update resources or trigger argo workflows on external events.  

### Complexity/Risks:
There is non trivial complexity involved in creating a separate project/repo/controller.  

### Alternatives:
We add polling primitives directly to existing kpack objects with the risk of introducing even more confusion about the intended role of our resources. 
