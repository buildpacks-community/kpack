### Context:

When kpack produces new builds today, a Reason is given which may be one of five values: COMMIT, TRIGGER, CONFIG, STACK, BUILDPACK

These one-word values are helpful for providing a basic level of context on why a new build has been created, but they don't provide enough information to inform users' decisions about what to do with the resulting build. Based on user interviews, we know they are asking themselves: "Does this new build have some update that affects my app? Should I run my regression tests on it? Should I promote it to prod?" 

When problems come up with an old build, they also ask themselves: "How do I know whats in this build that caused the problem? What makes this build different than the other ones that are working? Where should I start to troubleshoot that build?"

Providing more information would reduce the friction in users taking action on new builds, resolve issues with older builds, and increase users' app security by making sure more patched builds make it into production faster. 

### Goals:

Propose that a diff or reason-message, as appropriate, be printed in the context of the existing “Reason” attribute. This would apply only to this command

```
$ kp build status image-name
```

See mockups below. 

### Prior Art:
We have been thinking about different ways to surface the [Bill of Materials](https://github.com/buildpacks/pack/issues/378) for a build and how to show the delta between builds for a few months now. This is an evolution of that work. Although the `pack` cli renders the contents of a single BoM alone with the `inspect-image` command, I believe we don't should provide the BoM in this case, because this feature is intended to answer questions around WHY a build kicked off and whats different about the causes of the build, not the contents. It's important to address cause and contents separately because the dependencies involved are not 1:1 related.  A true diff of the contents would be better done using a different feature or standalone tool. 

### Complexity/Risks:
How does the user compare two non-adjacent builds? ex. builds 17 and 20

Users still have to do a significant amount of digging work to make this information actionable. For example, one developer told me he has to look through the Paketo buildpacks commits and/or diff the dpackage lists of the stacks to see what changed in order to decide whether he should cut a new release based on the new build.  

### Alternatives:
- Create a new diff command to encapsulate this functionality
- Do nothing and leave this diffing work to scripts and other small sharp tools

### Mockups

------------------------------

MOCKUP v3

Changes from last mockup:
- The feature now only applies to kp build status, not kp image status. 
- The diff is now printed directly beneath the reason, indented to be in-line with it. 
- Reason-message has been simplified to apply only to TRIGGER. 
-- For TRIGGER, reason-message applies because there is no diff to show. The only context we can give is who and when. 

Notes on implementation: 
- Lines preceded by a "-" should be printed in red (Red: \u001b[31m)
- Lines preceded by a "+" should to be printed in green (Green: \u001b[32m)
- Indentation should be preserved whenever possible in the diffs 

See mockup images attached in comment to thread 

------------------------------

**For Reason "TRIGGER"**

- Print the name of the user and the time () it was triggered as detail for the reason
- No diff

------------------------------

```
$kp build status petclinic
Image:      index.docker.io/username/instance@sha256:fsdgh39478th0g9tuisodfhgvns938e54iunwuiehfasf9wgiauwsfx
Status:     SUCCESS
Reason:     TRIGGER - A new build was manually triggered by [user] at [time].

Builder:    our-special-url.jfrog.io/instance/cluster-builder:default@sha4398ths9rdpfp9sghbne985-s9ed9hgbd9rz4sh98dxbfs
Run Image:  index.docker.io/paketobuildpacks/run@sha256:34978twgsui4789et0wh9384erhsg0e798riuhgs0er7iugshe98riugeh99rgaser89

Source:   Local Source 

BUILDPACK ID                            BUILDPACK VERSION
paketo-buildpacks/bellsoft-liberica     2.13.0
paketo-buildpacks/maven                 2.3.0
paketo-buildpacks/executable-jar        2.1.1
paketo-buildpacks/apache-tomcat         1.5.0
paketo-buildpacks/dist-zip              1.4.0
paketo-buildpacks/spring-boot           2.5.0
```

------------------------------

**For Reason "COMMIT"**

- Print a diff below the reason section that shows the change between the two revisions or the two branch names, whichever changed. 

------------------------------

```
$kp build status petclinic
Image:      index.docker.io/username/instance@sha256:fsdgh39478th0g9tuisodfhgvns938e54iunwuiehfasf9wgiauwsfx
Status:     SUCCESS
Reasons:    COMMIT 
            - Revision: 234890hpe9vjsr8gse98rghser
            + Revision: 43t789wghges87h540eq8378ge

Builder:    our-special-url.jfrog.io/instance/cluster-builder:default@sha4398ths9rdpfp9sghbne985-s9ed9hgbd9rz4sh98dxbfs
Run Image:  index.docker.io/paketobuildpacks/run@sha256:34978twgsui4789et0wh9384erhsg0e798riuhgs0er7iugshe98riugeh99rgaser89

Source:   Local Source 

BUILDPACK ID                            BUILDPACK VERSION
paketo-buildpacks/bellsoft-liberica     2.13.0
paketo-buildpacks/maven                 2.3.0
paketo-buildpacks/executable-jar        2.1.1
paketo-buildpacks/apache-tomcat         1.5.0
paketo-buildpacks/dist-zip              1.4.0
paketo-buildpacks/spring-boot           2.5.0
```

------------------------------

**For Reason "CONFIG"**

- Print a diff beneath the reason section
- Print the whole config, with diffs for specific lines, with "failedBuildHistoryLimit," "successBuildHistoryLimit," "apiVersion" removed (perhaps others also?)

------------------------------

```
$kp build status petclinic
Image:      index.docker.io/username/instance@sha256:fsdgh39478th0g9tuisodfhgvns938e54iunwuiehfasf9wgiauwsfx
Status:     SUCCESS
Reasons:    CONFIG
            spec:
              tag: gcr.io/PROJECT-NAME/app
              serviceAccount: SERVICE-ACCOUNT
              cacheSize: "CACHE-SIZE"
              source:
                git:
                  url: GIT-REPOSITORY-URL
                  revision: GIT-REVISION
              build:
                env:
                  - name: ENV-VAR-NAME
            -       value: ENV-VAR-VALUE-1
            +       value: ENV-VAR-VALUE-2                    
                resources:
                  limits:
                    cpu: CPU-LIMIT
                    memory: MEMORY-LIMIT
                  requests:
                    cpu: CPU-REQUEST
                  memory: MEMORY-REQUEST

Builder:    our-special-url.jfrog.io/instance/cluster-builder:default@sha4398ths9rdpfp9sghbne985-s9ed9hgbd9rz4sh98dxbfs
Run Image:  index.docker.io/paketobuildpacks/run@sha256:34978twgsui4789et0wh9384erhsg0e798riuhgs0er7iugshe98riugeh99rgaser89

Source:   Local Source 

BUILDPACK ID                            BUILDPACK VERSION
paketo-buildpacks/bellsoft-liberica     2.13.0
paketo-buildpacks/maven                 2.3.0
paketo-buildpacks/executable-jar        2.1.1
paketo-buildpacks/apache-tomcat         1.5.0
paketo-buildpacks/dist-zip              1.4.0
paketo-buildpacks/spring-boot           2.5.0
```

------------------------------

**For Reason "STACK"**

- Print a diff beneath the reason section
- Print the stack-name after the reason in order to give context 

------------------------------
```
$kp build status petclinic
Image:      index.docker.io/username/instance@sha256:fsdgh39478th0g9tuisodfhgvns938e54iunwuiehfasf9wgiauwsfx
Status:     SUCCESS
Reason:     STACK
            - RunImage: index.docker.io/paketobuildpacks/run@sha256:34978twgsui4789et0wh9384erhsg0e798riuhgs0er7iugshe98riugeh99rgaser892
            + RunImage: index.docker.io/paketobuildpacks/run@sha256:fh34578tg29wb645rwhs87e5hrw0e98jtfnws9e7w378qe49tr3g39n5gwe9u5hge549w
            
Builder:    our-special-url.jfrog.io/instance/cluster-builder:default@sha4398ths9rdpfp9sghbne985-s9ed9hgbd9rz4sh98dxbfs
Run Image:  index.docker.io/paketobuildpacks/run@sha256:fh34578tg29wb645rwhs87e5hrw0e98jtfnws9e7w378qe49tr3g39n5gwe9u5hge549w

Source:   Local Source 

BUILDPACK ID                            BUILDPACK VERSION
paketo-buildpacks/bellsoft-liberica     2.13.0
paketo-buildpacks/maven                 2.3.0
paketo-buildpacks/executable-jar        2.1.1
paketo-buildpacks/apache-tomcat         1.5.0
paketo-buildpacks/dist-zip              1.4.0
paketo-buildpacks/spring-boot           2.5.0
```

------------------------------

**For Reason "BUILDPACK"**

- Print a diff as part of the buildpacks table 

------------------------------

```
$kp build status petclinic
Image:      index.docker.io/username/instance@sha256:fsdgh39478th0g9tuisodfhgvns938e54iunwuiehfasf9wgiauwsfx
Status:     SUCCESS
Reason:     BUILDPACK
            - paketo-buildpacks/bellsoft-liberica   1.2.3 
            + paketo-buildpacks/bellsoft-liberica   1.2.4

Builder:    our-special-url.jfrog.io/instance/cluster-builder:default@sha4398ths9rdpfp9sghbne985-s9ed9hgbd9rz4sh98dxbfs
Run Image:  index.docker.io/paketobuildpacks/run@sha256:34978twgsui4789et0wh9384erhsg0e798riuhgs0er7iugshe98riugeh99rgaser89

Source:   Local Source 

BUILDPACK ID                            BUILDPACK VERSION
paketo-buildpacks/bellsoft-liberica     1.2.4
paketo-buildpacks/maven                 2.3.0
paketo-buildpacks/executable-jar        2.1.1
paketo-buildpacks/apache-tomcat         1.5.0
paketo-buildpacks/dist-zip              1.4.0
paketo-buildpacks/spring-boot           2.5.0
```

------------------------------

**For multiple reasons**

- Reasons are printed on new lines with a new line in between them, indented to be in-line with the other values. 

------------------------------

```
$kp build status petclinic
Image:      index.docker.io/username/instance@sha256:fsdgh39478th0g9tuisodfhgvns938e54iunwuiehfasf9wgiauwsfx
Status:     SUCCESS
Reason:     TRIGGER - A new build was manually triggered by [user] at [time].

            COMMIT
            - Revision: 234890hpe9vjsr8gse98rghser
            + Revision: 43t789wghges87h540eq8378ge

            CONFIG
            Build:
                env:
                - name: BP_JAVA_VERSION
            - value: 11.*
            - resources: {}
            - CacheSize: 1M
            + value: 14.*
            + resources:
            +   limits:
            +       cpu: 1M
            + CacheSize: 12M

            STACK
            - RunImage: index.docker.io/paketobuildpacks/run@sha256:34978twgsui4789et0wh9384erhsg0e798riuhgs0er7iugshe98riugeh99rgaser892
            + RunImage: index.docker.io/paketobuildpacks/run@sha256:fh34578tg29wb645rwhs87e5hrw0e98jtfnws9e7w378qe49tr3g39n5gwe9u5hge549w

            BUILDPACK
            - paketo-buildpacks/bellsoft-liberica   1.2.3 
            + paketo-buildpacks/bellsoft-liberica   1.2.4   

Builder:    our-special-url.jfrog.io/instance/cluster-builder:default@sha4398ths9rdpfp9sghbne985-s9ed9hgbd9rz4sh98dxbfs
Run Image:  index.docker.io/paketobuildpacks/run@sha256:34978twgsui4789et0wh9384erhsg0e798riuhgs0er7iugshe98riugeh99rgaser89

Source:     Local Source 

BUILDPACK ID                            BUILDPACK VERSION
paketo-buildpacks/bellsoft-liberica     1.2.4
paketo-buildpacks/maven                 2.3.0
paketo-buildpacks/executable-jar        2.1.1
paketo-buildpacks/apache-tomcat         1.5.0
paketo-buildpacks/dist-zip              1.4.0
paketo-buildpacks/spring-boot           2.5.0
```

------------------------------

**Build Logs**

------------------------------

```
===> PREPARE
Build info: 
-------------------
TRIGGER 
A new build was manually triggered by [user] at [time]

COMMIT
- Revision: 234890hpe9vjsr8gse98rghser
+ Revision: 43t789wghges87h540eq8378ge

CONFIG

spec:
 tag: gcr.io/PROJECT-NAME/app
 serviceAccount: SERVICE-ACCOUNT
 cacheSize: "CACHE-SIZE"
 source:
  git:
   url: GIT-REPOSITORY-URL
   revision: GIT-REVISION
 build:
  env:
   - name: ENV-VAR-NAME
-       value: ENV-VAR-VALUE-1
+       value: ENV-VAR-VALUE-2                    
  resources:
   limits:
    cpu: CPU-LIMIT
    memory: MEMORY-LIMIT
   requests:
    cpu: CPU-REQUEST
   memory: MEMORY-REQUEST

BUILDPACK
Buildpacks:
    - ID: paketo-buildpacks/bellsoft-liberica
-       Version: 1.2.3
+       Version: 1.2.4

STACK
- RunImage: index.docker.io/paketobuildpacks/run@sha256:34978twgsui4789et0wh9384erhsg0e798riuhgs0er7iugshe98riugeh99rgaser892
+ RunImage: index.docker.io/paketobuildpacks/run@sha256:fh34578tg29wb645rwhs87e5hrw0e98jtfnws9e7w378qe49tr3g39n5gwe9u5hge549w
```

-------------------

