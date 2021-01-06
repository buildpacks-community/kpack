**Problem:**
Currently, the lifecycle version used in kpack is not configurable, nor is it user upgradeable. Users must wait for a new kpack release to be able to upgrade the version of the lifecycle used by their builders.

**Outcome:**
We would like to decouple the lifecycle upgrade process from the kpack upgrade process, allowing users to upgrade the lifecycle on their own.

**Actions to take:**
A simple, monolithic approach that would keep a single lifecycle version for the whole cluster like it is today, but with the functionality for configuring that version exposed. The main benefit of doing something like this it will likely be the least intrusive on user workflow. The lifecycle would continue to be something that is more back of mind for users. We would ship a version of the lifecycle with each kpack release like we do today. Additionally, since kpack has to be compatible with the platform api of the lifecycle, keeping the lifecycle version tied to kpack instead of a resource on a builder seems to make more sense right now. We would implement this by adding a few features to kp:

1. Create a command in `kp` that would give users the ability to manually update their lifecycle:

	`kp lifecycle upgrade --image "gcr.io/my-registry/lifecycle:v0.9.2"
	
	Additionally, We could add the ability to update by providing the path to the windows and linux lifecycle downloads:
	
	`kp lifecycle upgrade --linux "https://github.com/buildpacks/lifecycle/releases/download/v0.9.2/lifecycle-v0.9.2+linux.x86-64.tgz" --windows "https://github.com/buildpacks/lifecycle/releases/download/v0.9.2/lifecycle-v0.9.2+windows.x86-64.tgz"`  
	
	In this case, kp would assemble the lifecycle image similar to how we do it in our kpack release.


2. Add the concept of lifecycle version to our descriptor files so that `kp import` can upgrade the lifecycle. 

Currently, the lifecycle image is passed to the kpack controller as an environment variable, so any change to the lifecycle image would require a kpack controller restart to propagate that change. We could get around this by monitoring the `lifecycle-image` config map for any changes using a knative ConfigMapInformer instead of providing the lifecycle as an environment variable to the controller.

We can implement compatibility checking to make sure that kpack can support the version of the lifecycle that is being used by creating a validating webhook that would make sure that the lifecycle in the config map has a platform api that was supported by kpack. This would require some metadata about the lifecycle to be included in the config map though. kp could fetch the image and add metadata containing the platform api to the `lifecycle-image` config map. The validating webhook would only attempt to validate the `lifecycle-image` config if the metadata was present.

Note: we would not want all images to be rebuilt on a lifecycle upgrade

**Complexity:**
The main point of complexity for the proposed solution is figuring out how to update the lifecycle without requiring the kpack controller to be restarted which we think can be solved using the monitored config map.

**Risks:**
Right now, the lifecycle is pretty hidden from the end user. By allowing it to be user upgradable, we would be making the user aware of it, which could create extra work from them on the ci/testing front.
