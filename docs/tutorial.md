#  kpack Tutorial

This tutorial will walk through creating a kpack [image](image.md) resource to build a docker image from source and allow kpack to keep the image up to date.  

###  Prerequisites
1. kpack is installed and available on a kubernetes cluster with a ClusterBuilder

    > Follow these docs to [install and setup kpack](install.md) 

1. kpack log utility is downloaded and available

    > Follow these docs to [setup log utility](logs.md)
     
###  Tutorial
1. Create a secret with push credentials for the docker registry that you plan on publishing images to with kpack.  

    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: tutorial-registry-credentials
      annotations:
        build.pivotal.io/docker: <registry-prefix>
    type: kubernetes.io/basic-auth
    stringData:
      username: <username>
      password: <password>
    ```
   
   > Note: The secret must be annotated with the registry prefix for its corresponding registry. For [dockerhub](https://hub.docker.com/) this should be `https://index.docker.io/v1/`. 
   For [GCR](https://cloud.google.com/container-registry/) this should be `gcr.io`. If you use GCR then the username can be `_json_key` and the password can be the JSON credentials you get from the GCP UI (under `IAM -> Service Accounts` create an account or edit an existing one and create a key with type JSON).
   
   Your secret configuration should look something like this:
   
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: tutorial-registry-credentials
     annotations:
       build.pivotal.io/docker: https://index.docker.io/v1/
   type: kubernetes.io/basic-auth
   stringData:
     username: sample-username
     password: sample-password
   ```
   
   or
   
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: tutorial-registry-credentials
     annotations:
       build.pivotal.io/docker: gcr.io
   type: kubernetes.io/basic-auth
   stringData:
     username: _json_key
     password: |
       {
         "type": "service-account",
         ... <rest of JSON from GCP>
       }
   ```
   
   Apply that credential to the cluster 
   
    ```bash
   kubectl apply -f secret.yaml
    ```
   
   > Note: Learn more about kpack secrets with the [kpack secret documentation](secrets.md) 

1. Create a service account that references the registry secret created above 

     ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: tutorial-service-account
    secrets:
      - name: tutorial-registry-credentials
     ```
    
    Apply that service account to the cluster 
   
     ```
     kubectl apply -f service-account.yaml
     ```

1. Apply a kpack image configuration 

    An image configuration is the specification for an image that kpack should build and manage. 
    
    We will create a sample image that builds with the default builder setup in the [installing documentation](./install.md).
    
    The example included here utilizes the [Spring Pet Clinic sample app](https://github.com/spring-projects/spring-petclinic). We encourage you to substitute it with your own application.           
      
    Create an image configuration:
    
    ```yaml
    apiVersion: build.pivotal.io/v1alpha1
    kind: Image
    metadata:
      name: tutorial-image
    spec:
      tag: <DOCKER-IMAGE>
      serviceAccount: tutorial-service-account
      cacheSize: "1.5Gi"
      builder:
        name: default
        kind: ClusterBuilder
      source:
        git:
          url: https://github.com/spring-projects/spring-petclinic
          revision: 82cb521d636b282340378d80a6307a08e3d4a4c4
    ```

   - Make sure to replace `<DOCKER-IMAGE>` with the registry you configured in step #2. Something like: your-name/app or gcr.io/your-project/app    
   - If you are using your application source, replace `source.git.url` & `source.git.revision`. 
    > Note: To use a private git repo follow the instructions in [secrets](secrets.md)

   Apply that image to the cluster 
    ```bash
    kubectl apply -f <name-of-image-file.yaml>
    ```
    
   You can now check the status of the image. 
   
   ```bash
   kubectl get images 
   ```
    
   You should see that the image has an unknown READY status as it currently building.
   
   ```
    NAME                  LATESTIMAGE   READY
    tutorial-image                      Unknown
    ```
    
    You can tail the logs for image that is currently building using the [logs utility](logs.md)
    
    > Note: The log utility will not exit when the build finishes. You will need to exit when it finishes.  
    ```
    logs -image tutorial-image  
    ``` 
    
    Once the image finishes building you can get the fully resolved built image with `kubectl get`
    
    ```
    kubectl get image tutorial-image
    ```  
    
    The output should look something like this:
    ```
    NAMESPACE   NAME                  LATESTIMAGE                                        READY
    test        tutorial-image        index.docker.io/your-project/app@sha256:6744b...   True
    ```
    
    The latest image is available to be used locally via `docker pull` and in a kubernetes deployment.   

1. Run the build app locally 

   Download the latest image available in step #4 and run it with docker.
    
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
    
1. kpack rebuilds
    
   We recommend updating the kpack image configuration with a CI/CD tool when new commits are ready to be built.
   > Note: You can also provide a branch or tag as the `spec.git.revision` and kpack will poll and rebuild on updates!  

   We can simulate an update from a CI/CD tool by updating the `spec.git.revision` on the image configured in step #3.
   
   If you are using your own application please push an updated commit and use the new commit sha. If you are using Spring Pet Clinic you can update the revision to: `4e1f87407d80cdb4a5a293de89d62034fdcbb847`.         
  
   Edit the image configuration with:
   ```
   kubectl edit image tutorial-image 
   ``` 
    
   You should see kpack schedule a new build by running:
   ```
   kubectl get builds
   ``` 
   You should see a new build with
   
   ```
   NAME                                IMAGE                                          SUCCEEDED
   tutorial-image-build-1-8mqkc       index.docker.io/your-name/app@sha256:6744b...   True
   tutorial-image-build-2-xsf2l                                                       Unknown
   ```

   You can tail the logs for the image with log utility used in step #4.
   
   ```
   logs -image tutorial-image -build 2  
   ```
   
   > Note: This second build should be notably faster because the buildpacks are able to leverage the cache from the previous build. 
    
1. kpack rebuilds with buildpack updates
    
    The next time the `cloudfoundry/cnb:bionic` is updated, kpack will detect if it contains buildpack updates to any of the buildpacks used by the tutorial image.
    If there is a buildpack update, kpack will automatically create a new build to rebuild your image.    
    
