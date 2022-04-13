#  kpack Tutorial

This tutorial will walk through creating a kpack [builder](builders.md) resource and a [image](image.md) resource to build a docker image from source and allow kpack rebuild the image with updates.  

###  Prerequisites
1. kpack is installed and available on a kubernetes cluster

    > Follow these docs to [install and setup kpack](install.md) 

1. kpack cli

    > Get the kp cli from the [github release](https://github.com/vmware-tanzu/kpack-cli/releases)
     
###  Tutorial
1. Create a secret with push credentials for the docker registry that you plan on publishing OCI images to with kpack.

   The easiest way to do that is with `kubectl create secret docker-registry`
   
    ```bash
    kubectl create secret docker-registry tutorial-registry-credentials \
        --docker-username=user \
        --docker-password=password \
        --docker-server=string \
        --namespace default
    ```
   
   > Note: The docker server must be the registry prefix for its corresponding registry. For [dockerhub](https://hub.docker.com/) this should be `https://index.docker.io/v1/`. 
   For [GCR](https://cloud.google.com/container-registry/) this should be `gcr.io`. If you use GCR then the username can be `_json_key` and the password can be the JSON credentials you get from the GCP UI (under `IAM -> Service Accounts` create an account or edit an existing one and create a key with type JSON).
   
   Your secret create should look something like this:
   
    ```bash
    kubectl create secret docker-registry tutorial-registry-credentials \
        --docker-username=my-dockerhub-username \
        --docker-password=my-dockerhub-password \
        --docker-server=https://index.docker.io/v1/ \
        --namespace default
    ```
   
   > Note: Learn more about kpack secrets with the [kpack secret documentation](secrets.md) 

2. Create a service account that references the registry secret created above

    ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: tutorial-service-account
      namespace: default
    secrets:
    - name: tutorial-registry-credentials
    imagePullSecrets:
    - name: tutorial-registry-credentials
    ```
    
    Apply that service account to the cluster 
   
     ```bash
     kubectl apply -f service-account.yaml
     ```

3. Create a cluster store configuration
    
   A store resource is a repository of [buildpacks](http://buildpacks.io/) packaged in [buildpackages](https://buildpacks.io/docs/buildpack-author-guide/package-a-buildpack/) that can be used by kpack to build OCI images. Later in this tutorial you will reference this store in a Builder configuration.
    
    We recommend starting with buildpacks from the [paketo project](https://github.com/paketo-buildpacks). The example below pulls in java and nodejs buildpacks from the paketo project. 
    
    ```yaml
    apiVersion: kpack.io/v1alpha2
    kind: ClusterStore
    metadata:
      name: default
    spec:
      sources:
      - image: gcr.io/paketo-buildpacks/java
      - image: gcr.io/paketo-buildpacks/nodejs
    ```
   
    Apply this store to the cluster 
  
    ```bash
    kubectl apply -f store.yaml
    ```

    > Note: Buildpacks are packaged and distributed as buildpackages which are docker images available on a docker registry. Buildpackages for other languages are available from [paketo](https://github.com/paketo-buildpacks).

4. Create a cluster stack configuration
    
    A stack resource is the specification for a [cloud native buildpacks stack](https://buildpacks.io/docs/concepts/components/stack/) used during build and in the resulting app image. 
    
    We recommend starting with the [paketo base stack](https://github.com/paketo-buildpacks/stacks) as shown below:
    
    ```yaml
    apiVersion: kpack.io/v1alpha2
    kind: ClusterStack
    metadata:
      name: base
    spec:
      id: "io.buildpacks.stacks.bionic"
      buildImage:
        image: "paketobuildpacks/build:base-cnb"
      runImage:
        image: "paketobuildpacks/run:base-cnb"
    ```

    Apply this stack to the cluster 
  
    ```bash
    kubectl apply -f stack.yaml
    ```

5. Create a Builder configuration

    A Builder is the kpack configuration for a [builder image](https://buildpacks.io/docs/concepts/components/builder/) that includes the stack and buildpacks needed to build an OCI image from your app source code.
    
    The Builder configuration will write to the registry with the secret configured in step one and will reference the stack and store created in step three and four. The builder order will determine the order in which buildpacks are used in the builder.
        
    ```yaml
    apiVersion: kpack.io/v1alpha2
    kind: Builder
    metadata:
      name: my-builder
      namespace: default
    spec:
      serviceAccountName: tutorial-service-account
      tag: <DOCKER-IMAGE-TAG>
      stack:
        name: base
        kind: ClusterStack
      store:
        name: default
        kind: ClusterStore
      order:
      - group:
        - id: paketo-buildpacks/java
      - group:
        - id: paketo-buildpacks/nodejs
    ```

    - Replace `<DOCKER-IMAGE-TAG>` with a valid image tag that exists in the registry you configured with the `--docker-server` flag when creating a Secret in step #1. The tag should be something like: your-name/builder or gcr.io/your-project/builder
 
    Apply this builder to the cluster 
   
     ```bash
     kubectl apply -f builder.yaml
     ```

6. Apply a kpack image resource

    An image resource is the specification for an OCI image that kpack should build and manage.
    
    We will create a sample image resource that builds with the builder created in step five.
    
    The example included here utilizes the [Spring Pet Clinic sample app](https://github.com/spring-projects/spring-petclinic). We encourage you to substitute it with your own application.           
      
    Create an image resource:
    
    ```yaml
    apiVersion: kpack.io/v1alpha2
    kind: Image
    metadata:
      name: tutorial-image
      namespace: default
    spec:
      tag: <DOCKER-IMAGE-TAG>
      serviceAccountName: tutorial-service-account
      builder:
        name: my-builder
        kind: Builder
      source:
        git:
          url: https://github.com/spring-projects/spring-petclinic
          revision: 82cb521d636b282340378d80a6307a08e3d4a4c4
    ```

   - Replace `<DOCKER-IMAGE-TAG>` with a valid image tag that exists in the registry you configured with the `--docker-server` flag when creating a Secret in step #1. Something like: your-name/app or gcr.io/your-project/app
   - If you are using your application source, replace `source.git.url` & `source.git.revision`. 
    > Note: To use a private git repo follow the instructions in [secrets](secrets.md)

   Apply that image resource to the cluster

    ```bash
    kubectl apply -f image.yaml
    ```
    
   You can now check the status of the image resource.
   
   ```bash
   kubectl -n default get images
   ```
    
   You should see that the image resource has an unknown READY status as it is currently building.
   
   ```
    NAME                  LATESTIMAGE   READY
    tutorial-image                      Unknown
    ```
    
    You can tail the logs for image that is currently building using the [kp cli](https://github.com/vmware-tanzu/kpack-cli/blob/main/docs/kp_build_logs.md)
    
    ```
    kp build logs tutorial-image -n default
    ``` 
    
    Once the image resource finishes building you can get the fully resolved built OCI image with `kubectl get`
    
    ```
    kubectl -n default get image tutorial-image
    ```  
    
    The output should look something like this:
    ```
    NAMESPACE   NAME                  LATESTIMAGE                                        READY
    default     tutorial-image        index.docker.io/your-project/app@sha256:6744b...   True
    ```
    
    The latest OCI image is available to be used locally via `docker pull` and in a kubernetes deployment.

8. Run the built app locally

   Download the latest OCI image available in step #6 and run it with docker.
    
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
    
9. kpack rebuilds
    
   We recommend updating the kpack image resource with a CI/CD tool when new commits are ready to be built.
   > Note: You can also provide a branch or tag as the `spec.git.revision` and kpack will poll and rebuild on updates!  

   We can simulate an update from a CI/CD tool by updating the `spec.git.revision` on the image resource used in step #6.
   
   If you are using your own application please push an updated commit and use the new commit sha. If you are using Spring Pet Clinic you can update the revision to: `4e1f87407d80cdb4a5a293de89d62034fdcbb847`.         
  
   Edit the image resource with:
   ```
   kubectl -n default edit image tutorial-image 
   ``` 
    
   You should see kpack schedule a new build by running:
   ```
   kubectl -n default get builds
   ``` 
   You should see a new build with
   
   ```
   NAME                                IMAGE                                          SUCCEEDED
   tutorial-image-build-1-8mqkc       index.docker.io/your-name/app@sha256:6744b...   True
   tutorial-image-build-2-xsf2l                                                       Unknown
   ```

   You can tail the logs for the image with the kp cli used in step #6.
   
   ```
   kp build logs tutorial-image -n default -b 2
   ```
   
   > Note: This second build should be notably faster because the buildpacks are able to leverage the cache from the previous build. 
    
10. Next steps
    
     The next time new buildpacks are added to the store, kpack will automatically rebuild the builder. If the updated buildpacks were used by the tutorial image resource, kpack will automatically create a new build to rebuild your OCI image.

     Now you can configure kpack to sign your images with cosign with the instructions in [kpack with Cosign Tutorial](cosign-tutorial.md).
