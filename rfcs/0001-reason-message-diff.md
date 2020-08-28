### Context:

When kpack produces new builds today, a Reason is given which may be one of five values: COMMIT, TRIGGER, CONFIG, STACK, BUILDPACK

These one-word values are helpful for providing a basic level of context on why a new build has been created, but they don't provide enough information to inform users' decisions about what to do with the resulting build. They may be asking themselves: "Does this new build have some update that affects my app? Should I run my regression tests on it? Should I promote it to prod?" 

Providing more information would reduce the friction in users taking action on new builds and increase customers' app security by making sure more patched builds make it into production faster. 

### Goals:

Propose an additional attribute, “Reason-Message” be added to the build object, which can be printed in the context of the existing “Reason” attribute. Contextual information can be provided in the Reason-Message and a diff may also be printed as appropriate. This “Reason-Message” would be printed for these two commands:

$ kp build status image-name
$ kp image status image-name

The format in which it would be printed:

Reason: [Reason-Code] - [Reason-Message]

Examples: 
**For Reason "COMMIT"**
- Ex. Reason: COMMIT - Received a new commit with ID [truncated-SH] from branch [branch-name]. 

**For Reason "CONFIG"**
- Ex. Reason: CONFIG - The [image-name] image config was updated. See diff below.
[output of diffing library on the two image config files]

**For Reason "BUILDPACK"**
- Ex. Reason: BUILDER - The buildpacks used by your app have changed. See diff below. 
[output of diffing library on the two Bill of Materials' for the builds]

**For Reason "STACK"**
- Ex. Reason: STACK - Stack-name changed from dockerhub/sha1 to dockerhub/sha2

**For Reason "TRIGGER"**
- Ex. Reason: TRIGGER - A new build was manually triggered by [user] at [time]

### Prior Art:
We have been thinking about different ways to surface the [Bill of Materials](https://github.com/buildpacks/pack/issues/378) for a build and how to show the delta between builds for a few months now. This is an evolution of that work. Although the `pack` cli renders the contents of a single BoM alone with the `inspect-image` command, I believe we don't need to provide a BoM alone, because the purpose of kpack is to iteratively build images, and therefore build BoMs should be placed in context as much as possible.

### Complexity/Risks:
How does the output render when there are two Reasons for a new build?

How does the user compare two non-adjacent builds? ex. builds 17 and 20

Users still have to do a significant amount of digging work to make this information actionable. For example, one developer told me he has to look through the Paketo buildpacks commits and/or diff the dpackage lists of the stacks to see what changed in order to decide whether he should cut a new release based on the new build. 



### Alternatives:

- Create a new diff command to encapsulate this functionality
- Leave this diffing work to scripts and other small sharp tools
