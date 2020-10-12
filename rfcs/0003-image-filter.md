### Context:

When kpack produces a build, certain attributes are surfaced to the end-user that serve to provide additional context about the build and why it was initiated.   Some of this metadata intends to provide end-users with a summary of how the image was built.  These fields include `Image`, `Pod Name`, `Builder`, `Run Image`, `Source`, `Url`, and `Revision`.  

We provide additional fields in the kpack build object that are geared towards reassuring the user that a successful build occurred as well as why the build was initiated. Because of their usefulness in conveying this information to the user, the build’s `STATUS` and `REASON` attributes are surfaced prominently in the `kp build list` table.  

### Problem:

Some users of the kpack CLI have complained that it is difficult to get a broad view of kpack images and builds at scale at a glance without needing to further drill down to discover more details about images or builds. This problem led to specific feature requests from Jason Morgan (VMware Solutions Engineer) to unlink the build list table from images and enable users to filter builds by their `REASON` and `STATUS` regardless of their parent image.   

### Use Cases:

Based on evidence provided by Jason and conducting user interviews with VMware field early-adopters, I perceive the following workflows to pose more complexity than is acceptable when a user is managing many images using the kpack CLI.

* A dev ops user updates a stack that is used by most if not all applications built by kpack and wants to validate that the images are in a ready state
* A dev ops user updates a store with new buildpacks and wants to validate that the affected images rebuilt successfully
* A dev ops user checking in to make sure that most Images are in a ready state so they can be sure that kpack is being used properly 

### Prior Art:

* Build Reason RFC - Addresses the problem of bringing clarity to why another build was triggered
    * This RFC is trying to solve the somewhat related problem of providing clarity to the user after an action is performed that causes rebuilds/rebases
    * https://github.com/sampeinado/kpack/blob/master/rfcs/0002-reason-message-diff.md

* Issue that stemmed from the user interview with Jason Morgan in which he requested the ability to filter the `kp build list` table https://github.com/vmware-tanzu/kpack-cli/issues/79 
* Kubernetes Field Selectors https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/

### Complexity/Risk

* By introducing a new column in `kp image list`, we may encounter a similar width problem as we did with the builds list table

### Alternatives:

* Users parse their image list as it currently exists and scroll through it until they find the images with the attributes they care about
* Usage of kubectl to return images with certain attributes

### Outcome:

Propose an additional feature or set of features that enable users to query kpack images based on their `status` and `latest-reason` such that it is easier for users to perform the workflows in the “Use Cases” section.  In order to make this filtration feature more effective, users should be able to list all images in the cluster (dependent on the user's RBAC permissions). Following a round of feedback during a working group meeting on 10/9, I am additionally proposing the inclusion of `builder` and `clusterbuilder` flags.  This modification addresses the concern that that we expect kpack images to be updating frequently, thereby making it difficult for the end-user to verify a stack or store update if the images have updated more recently based on different `REASON`.  These flags would effectively enable the user to display the last time and image was rebuilt for `Reason` `BUILDPACK` or `STACK` 

Although the feature request stated in the “Problem” section is to query builds, it is probably more productive to query images because end-users most likely care about the current state of their application build and not builds that happened in the past.  Additionally, querying based on builds would pose a technical problem as the CLI would need to present builds and their build numbers outside the context of their associated image.  


### Mockups

Below I will cover specific scenarios made possible by a filtering feature. All the following scenarios are derived from this proposed modification to the `kp image list` command:

```
kp imag list --status <string> --latest-reason <string1, string2 ...>
```

Rather than adding an additional subcommand to the `kp image list` path like `filter` or `query`, I propose adding an additional `LATEST REASON` column to the existing output of the `kp image list` table. This would also allow for future design extension. If we wanted to add additional filtering capabilities, we would just create new attribute flags with values as arguments.

Note: The mockups are not meant to provide sample output for all possible scenarios when using this command. Rather they focus on the workflows described in the "Use Cases" section and some non-obvious scenarios that are made possible by introduction of the new command.

Note: `SHA1` is used in the mockups that display images with a `ready` status so that rows don't wrap in the body of the PR

------------------------------

**Sample Help Output**

```
$ kp image list -h

Prints a table of the most important information about images in the provided namespace.

The namespace defaults to the kubernetes current-context namespace.

Apply flags to filter images displayed in the table 

Flags:

-A,  --all-namespaces
     --builder string
-cb, --clusterbuilder string
     --latest-reason string1, string 2, ...   possible arguments: commit, trigger, config, stack, buildpack
-h,  --help                                   help for list
-n,  --namespace string                       kubernetes namespace
--status string, string1                      possible arguments: ready, not-ready, unknown 
```

------------------------------

**Print Image Configs That Are Not Ready**

```
$kp image list --status not-ready -A

NAME             READY    LATEST IMAGE     LATEST REASON

mg-test-image    False                     CONFIG
mg-test-image2   False                     BUILDPACK
mg-test-image3   False                     STACK
mg-test-image4   False                     TRIGGER
mg-test-image5   False                     COMMIT 
...
```

------------------------------

**Print Image Configs That Are Ready**

```
$kp image list --status ready -A

NAME             READY    LATEST IMAGE                                                                                           LATEST REASON

mg-test-image    True     gcr.io/cf-build-service-dev-219913/test/mg-test-image@sha1:5163C01DEAF54C2A814C71A2A214A241F3BF680B    CONFIG
mg-test-image2   True     gcr.io/cf-build-service-dev-219913/test/mg-test-image2@sha1:B75D2E59189B813285726A36CD0A737D2720832C   BUILDPACK
mg-test-image3   True     gcr.io/cf-build-service-dev-219913/test/mg-test-image3@sha1:32EAA227FBEC2CBF6C1D3C3A84062FB2C41A55D1   STACK
mg-test-image4   True     gcr.io/cf-build-service-dev-219913/test/mg-test-image4@sha1:B2F2F07F4583C3870131B8A3447C7D04B046A0E5   TRIGGER
mg-test-image5   True     gcr.io/cf-build-service-dev-219913/test/mg-test-image5@sha1:EF305A16C96FF55AA3A9A764FD942D6DDA5B34AF   COMMIT 
...
```

------------------------------

**Print Image Configs With A Certain Reason**

```
$kp image list --latest-reason stack -A

NAME             READY    LATEST IMAGE                                                                                            LATEST REASON

mg-test-image    False                                                                                                            STACK
mg-test-image2   True     gcr.io/cf-build-service-dev-219913/test/mg-test-image2@sha1:B75D2E59189B813285726A36CD0A737D2720832C    STACK
mg-test-image3   False                                                                                                            STACK
mg-test-image4   True     gcr.io/cf-build-service-dev-219913/test/mg-test-image4@sha1:B2F2F07F4583C3870131B8A3447C7D04B046A0E5    STACK
...
```

------------------------------

**Print Image Configs That Are Ready And Have A Stack or Buildpack Latest Reason**

```
$kp image list --status ready --latest-reason stack, buildpack -A

NAME             READY    LATEST IMAGE                                                                                              LATEST REASON

mg-test-image1   True     gcr.io/cf-build-service-dev-219913/test/mg-test-image@sha1:13D5582884803A7755CEEDCB7478485498C4F1C4       STACK
mg-test-image2   True     gcr.io/cf-build-service-dev-219913/test/mg-test-image2@sha1:2E46CC4B032552F76ADD7622F63D953F330C428A      BUILDPACK
mg-test-image3   True     gcr.io/cf-build-service-dev-219913/test/mg-test-image3@sha1:32EAA227FBEC2CBF6C1D3C3A84062FB2C41A55D1      STACK
mg-test-image4   True     gcr.io/cf-build-service-dev-219913/test/mg-test-image4@sha1:B2F2F07F4583C3870131B8A3447C7D04B046A0E5      BUILDPACK
mg-test-image5   True     gcr.io/cf-build-service-dev-219913/test/mg-test-image5@sha1:EF305A16C96FF55AA3A9A764FD942D6DDA5B34AF      STACK
...
```

------------------------------

**Image Configs Are Not Ready Because Adding A New Buildpack To The Default Store Caused a Breaking Change**

```   
$kp image list --clusterbuilder default -A

NAME             READY     LATEST IMAGE                                                                                              LATEST REASON

mg-test-image1   False     gcr.io/cf-build-service-dev-219913/test/mg-test-image@sha1:13D5582884803A7755CEEDCB7478485498C4F1C4       BUILDPACK
mg-test-image2   False     gcr.io/cf-build-service-dev-219913/test/mg-test-image2@sha1:2E46CC4B032552F76ADD7622F63D953F330C428A      BUILDPACK
mg-test-image3   False     gcr.io/cf-build-service-dev-219913/test/mg-test-image3@sha1:32EAA227FBEC2CBF6C1D3C3A84062FB2C41A55D1      BUILDPACK
mg-test-image4   False     gcr.io/cf-build-service-dev-219913/test/mg-test-image4@sha1:B2F2F07F4583C3870131B8A3447C7D04B046A0E5      BUILDPACK
mg-test-image5   False     gcr.io/cf-build-service-dev-219913/test/mg-test-image5@sha1:EF305A16C96FF55AA3A9A764FD942D6DDA5B34AF      BUILDPACK
...
```

**Multiple Latest Reasons Are Listed Even If User Did Not Filter For Them**

```
$kp image list --status ready --latest-reason config -A 

NAME             READY    LATEST IMAGE                                                                                              LATEST REASON

mg-test-image1   TRUE     gcr.io/cf-build-service-dev-219913/test/mg-test-image@sha1:13D5582884803A7755CEEDCB7478485498C4F1         CONFIG, BUILDPACK
...
```
