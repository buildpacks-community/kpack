# kpack Tutorial using the kp cli

This tutorial will walk through creating a kpack [builder](builder.md) resource and a [image](image.md) resource to
build an OCI image from source and allow kpack rebuild the OCI image with updates while using the `kp` cli instead of `kubectl`.

### Prerequisites

1. kpack is installed and available on a kubernetes cluster

   > Follow these docs to [install and setup kpack](install.md).

2. kpack cli

   > Get the kp cli. Installation steps can be found [here](https://github.com/vmware-tanzu/kpack-cli#installation).

### Tutorial

1. Configure the registry credentials where the cluster store and cluster stack images will be stored.`kp` supports two ways to provide registry credentials to upload OCI images
   1. Set environment variables, see [auth](https://github.com/buildpacks-community/kpack-cli/blob/main/docs/auth.md).
      ```bash
      export KP_REGISTRY_HOSTNAME=<REGISTRY-HOSTNAME>
      export KP_REGISTRY_USERNAME=<REGISTRY-USERNAME>
      export KP_REGISTRY_PASSWORD=<REGISTRY-PASSWORD>
      ````
   2. or log in using `docker login`

       ```bash
       docker login <REGISTRY-HOSTNAME> -u <REGISTRY-USERNAME>
      ```
      >    Note: The `<REGISTRY-HOSTNAME>` must be the registry URL. For [dockerhub](https://hub.docker.com/) use `index.docker.io`, [GCR](https://cloud.google.com/container-registry/) use `gcr.io`, etc.

2. Configure the kp default repository where the cluster store and cluster stack images will be saved. 
   ```bash
   kp config default-repository <REPOSITORY>
   ```
   >    Note: The `<REPOSITORY>` must be a location in the registry that can be written to with the credentials from the previous step.
   >    - For [dockerhub](https://hub.docker.com/) this should be `my-username/my-repo`.
   >    - For [GCR](https://cloud.google.com/container-registry/) this should be `gcr.io/my-project/my-repo`.

3. Create a cluster store

   A store resource is a repository of [buildpacks](http://buildpacks.io/) packaged
   in [buildpackages](https://buildpacks.io/docs/buildpack-author-guide/package-a-buildpack/) that can be used by kpack
   to build OCI images. Later in this tutorial you will reference this store in a Builder configuration.

   We recommend starting with buildpacks from the [paketo project](https://github.com/paketo-buildpacks). The example
   below pulls in java buildpack from the paketo project. If you would like to use a different language you can select a different buildpack.

    ```
    kp clusterstore save default -b gcr.io/paketo-buildpacks/java
    ```

   > Note: Buildpacks are packaged and distributed as buildpackages which are OCI images available on a docker registry. Buildpackages for other languages are available from [paketo](https://github.com/paketo-buildpacks).

4. Create a cluster stack

   A stack resource is the specification for
   a [cloud native buildpacks stack](https://buildpacks.io/docs/concepts/components/stack/) used during build and in the
   resulting app image.

   We recommend starting with the [paketo base stack](https://github.com/paketo-buildpacks/stacks) as shown below:

    ```
    kp clusterstack save base --build-image paketobuildpacks/build:base-cnb --run-image paketobuildpacks/run:base-cnb
    ```

5. Set the default service account. `kp` will automatically manage this service account to track secrets created `kp secret create`.
   Here we are using the default service account that is automatically created by kubernetes in each namespace.
    
   ```bash
    kp config default-service-account default
    ```

6. Create a secret with push credentials for the registry that you plan on publishing OCI images to with kpack. 
   This secret needs to be located in the same namespace as the builder that will be created in the next step.

   The easiest way to do that is with `kp secret create`

    ```bash
    kp secret create tutorial-registry-credentials \
       --registry <REGISTRY-HOSTNAME> \
       --registry-user <REGISTRY-USER> \
       -n default
    ```

   > Note: The `<REGISTRY-HOSTNAME>` must be the registry prefix for its corresponding registry
   > - For [dockerhub](https://hub.docker.com/) `kp` offers a simplified way to create a dockerhub secret with a `--dockerhub` flag. The registry in the secret should be `https://index.docker.io/v1/`.
   > - For [GCR](https://cloud.google.com/container-registry/), `kp` also offers a simplified way to create a GCR secret with a `--gcr` flag that followed by a path to a json file containing the service account password. The registry in the secret should be `gcr.io`.

   Your secret create should look something like this:

    ```bash
    kp secret create tutorial-registry-credentials \
       --registry my-registry.io \
       --registry-user my-registry-username \
       -n default
    ```
   or
    ```bash
    kp secret create tutorial-registry-credentials \
       --gcr service_account_password.json \
       -n default
    ```

   > Note: Learn more about kpack secrets with the [kpack secret documentation](secrets.md)

7. Create a Builder

   A Builder is the kpack configuration for a [builder image](https://buildpacks.io/docs/concepts/components/builder/)
   that includes the stack and buildpacks needed to build an OCI image from your app source code.

   The Builder configuration will write to the registry with the secret configured in the previous step and will reference the
   stack and store created previously. The builder order will determine the order in which buildpacks are
   used in the builder.

    ```
    kp builder save my-builder \
      --tag <IMAGE-TAG> \
      --stack base \
      --store default \
      --buildpack paketo-buildpacks/java \
      -n default
    ```

    - Replace `<IMAGE-TAG>` with a valid image tag that exists in the registry you configured with the `--registry` flag when creating a Secret in the previous step. 
      The tag should be something like: `your-name/builder` or `gcr.io/your-project/your-repo/builder`.

8. Create a kpack image resource

   An image resource is the specification for an OCI image that kpack should build and manage.

   We will create a sample image resource that builds with the builder created in step five.

   The example included here utilizes
   the [Spring Pet Clinic sample app](https://github.com/spring-projects/spring-petclinic). We encourage you to
   substitute it with your own application.

   Create an image resource:

    ```yaml
    kp image save tutorial-image \
      --tag <IMAGE-TAG> \
      --git https://github.com/spring-projects/spring-petclinic \
      --git-revision 82cb521d636b282340378d80a6307a08e3d4a4c4 \
      --builder my-builder \
      -n default
    ```

   - Make sure to replace `<IMAGE-TAG>` with the tag in the registry of the secret you configured in step #5. Something like:
     your-name/app or gcr.io/your-project/app
   - If you are using your application source, replace `--git` & `--git-revision`.
   > Note: To use a private git repo follow the instructions in [secrets](secrets.md)

   You can now check the status of the image resource.

   ```bash
   kp image status tutorial-image -n default
   ```

   You should see that the image resource has a status Building as it is currently building.

    ```
    Status:         Building
    Message:        --
    LatestImage:    --
    
    Source
    Type:        GitUrl
    Url:         https://github.com/spring-projects/spring-petclinic
    Revision:    82cb521d636b282340378d80a6307a08e3d4a4c4
    
    Builder Ref
    Name:    base
    Kind:    Builder
    
    Last Successful Build
    Id:              --
    Build Reason:    --
    
    Last Failed Build
    Id:              --
    Build Reason:    --
    ```

   You can tail the logs for image resource that is currently building using
   the [kp cli](https://github.com/vmware-tanzu/kpack-cli/blob/main/docs/kp_build_logs.md)

    ```
    kp build logs tutorial-image -n default
    ``` 

   Once the image resource finishes building you can get the fully resolved built OCI image with `kp image status`

    ```bash
    kp image status tutorial-image -n default
    ```

   The output should look something like this:
    ```
    Status:         Ready
    Message:        --
    LatestImage:    index.docker.io/your-project/app@sha256:6744b...
    
    Source
    Type:        GitUrl
    Url:         https://github.com/spring-projects/spring-petclinic
    Revision:    82cb521d636b282340378d80a6307a08e3d4a4c4
    
    Builder Ref
    Name:    base
    Kind:    Builder
    
    Last Successful Build
    Id:              1
    Build Reason:    BUILDPACK
    Git Revision:    82cb521d636b282340378d80a6307a08e3d4a4c4
    
    BUILDPACK ID                           BUILDPACK VERSION    HOMEPAGE
    paketo-buildpacks/ca-certificates      2.4.0                https://github.com/paketo-buildpacks/ca-certificates
    paketo-buildpacks/bellsoft-liberica    8.4.0                https://github.com/paketo-buildpacks/bellsoft-liberica
    paketo-buildpacks/gradle               5.5.0                https://github.com/paketo-buildpacks/gradle
    paketo-buildpacks/executable-jar       5.2.0                https://github.com/paketo-buildpacks/executable-jar
    paketo-buildpacks/apache-tomcat        6.1.0                https://github.com/paketo-buildpacks/apache-tomcat
    paketo-buildpacks/dist-zip             4.2.0                https://github.com/paketo-buildpacks/dist-zip
    paketo-buildpacks/spring-boot          4.5.0                https://github.com/paketo-buildpacks/spring-boot
    
    Last Failed Build
    Id:              --
    Build Reason:    --
    ```

   The latest built OCI image is available to be used locally via `docker pull` and in a kubernetes deployment.

9. Run the built app locally

   Download the latest built OCI image available in step #6 and run it with docker.

   ```bash
   docker run -p 8080:8080 <latest-image-with-digest>
   ```

   You should see the java app start up:
   ```
       
              |\      _,,,--,,_
             /,`.-'`'   ._  \-;;,_
    _______ __|,4-  ) )_   .;.(__`'-'__     ___ __    _ ___ _______
    |       | '---''(_/._)-'(_\_)   |   |   |   |  |  | |   |       |
    |    _  |    ___|_     _|       |   |   |   |   |_| |   |       | __ _ _
    |   |_| |   |___  |   | |       |   |   |   |       |   |       | \ \ \ \
    |    ___|    ___| |   | |      _|   |___|   |  _    |   |      _|  \ \ \ \
    |   |   |   |___  |   | |     |_|       |   | | |   |   |     |_    ) ) ) )
    |___|   |_______| |___| |_______|_______|___|_|  |__|___|_______|  / / / /
    ==================================================================/_/_/_/
    
    :: Built with Spring Boot :: 2.2.2.RELEASE
   ``` 

10. kpack rebuilds

    We recommend updating the kpack image resource with a CI/CD tool when new commits are ready to be built.
   > Note: You can also provide a branch or tag as the `spec.git.revision` and kpack will poll and rebuild on updates!

    We can simulate an update from a CI/CD tool by updating the `spec.git.revision` on the image resource configured in step #6.

    If you are using your own application please push an updated commit and use the new commit sha. If you are using
    Spring Pet Clinic you can update the revision to: `4e1f87407d80cdb4a5a293de89d62034fdcbb847`.

    Edit the image resource with:
    ```
    kp image save tutorial-image --git-revision 4e1f87407d80cdb4a5a293de89d62034fdcbb847 -n default
    ``` 

    You should see kpack schedule a new build by running:
    ```
    kp build list tutorial-image -n default
    ``` 
    You should see a new build with

    ```
    BUILD    STATUS     IMAGE                                            REASON
    1        SUCCESS    index.docker.io/your-name/app@sha256:6744b...    BUILDPACK
    2        BUILDING                                                    CONFIG
    ```

    You can tail the logs for the image resource with the kp cli used in step #6.

    ```
    kp build logs tutorial-image -n default
    ```

   > Note: This second build should be notably faster because the buildpacks are able to leverage the cache from the previous build.

11. Next steps

    The next time new buildpacks are added to the store, kpack will automatically rebuild the builder. If the updated
    buildpacks were used by the tutorial image resource, kpack will automatically create a new build to rebuild your OCI image.
    in the registry you configured with the `--registry` flag when creating a Secret in step #1. The tag should be something like: `your-name/builder` or `gcr.io/your-project/your-repo/builder`.