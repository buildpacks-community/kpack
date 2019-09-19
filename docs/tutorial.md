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
      name: registry-credentials
      annotations:
        build.pivotal.io/docker: <registry>
    type: kubernetes.io/basic-auth
    stringData:
      username: <username>
      password: <password>
    ```
   
   > Note: The secret must be annotated with the registry prefix for its corresponding registry. For [dockerhub](https://hub.docker.com/) this should be `index.docker.io`. 
   For GCR this should be `gcr.io`.
   
   Your secret configuration should look something like this:
   
   ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: tutorial-registry-credentials
      annotations:
        build.pivotal.io/docker: index.docker.io
    type: kubernetes.io/basic-auth
    stringData:
      username: sample-username
      password: sample-password
   ```
   
   Apply that credential to the cluster 
   
    ```bash
   kubectl apply -f <name-of-secret-file.yaml>
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
     kubectl apply -f <name-of-service-account-file.yaml>
     ```

1. Fork the buildpacks sample app
    
    Navigate to https://github.com/buildpack/sample-java-app and fork the repo to your account.
    
    You will use this fork to build an app with kpack and watch it update when pushes are made to your fork.   

1. Apply kpack image configuration 

    An image configuration is the specification for an image that kpack should build and manage. For more info check out image documentation. We will create a sample image that builds with the default builder setup in installing.      
      
    Create an image configuration:
    
    ```yaml
    apiVersion: build.pivotal.io/v1alpha1
    kind: Image
    metadata:
      name: tutorial-image
    spec:
      tag: <REGISTRY-TAG>
      serviceAccount: tutorial-service-account
      builder:
        name: default-builder
        kind: ClusterBuilder
      source:
        git:
          url: <YOUR-BULIDPACK-SAMPLE-APP-FORK>
          revision: master
    ```

   - Make sure to replace <REGISTRY> with the registry you configured in step #2.     
   - Make sure to replace <YOUR-GITHUB-URL> with a publicly accessible github url
    
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
    NAMESPACE   NAME                  LATESTIMAGE                                                                                       READY
    test        tutorial-image        gcr.io/project-name/app@sha256:6744b3b24a7ab8d2b45d7673313c180daccb534b3d931369d0aa9805712f34b8   True
    ```
    
    The latest image is available to be used in locally via `docker pull` and in a kubernetes deployment.   

1. Run the build app locally 

    Download the latest image available in step #4 and docker run it
    
   ```bash
   docker run -p 8080:8080 <latest-image-with-digest>
   ```
   
   You should see the java app start up:
   ```
       |'-_ _-'|       ____          _  _      _                      _             _
       |   |   |      |  _ \        (_)| |    | |                    | |           (_)
        '-_|_-'       | |_) | _   _  _ | |  __| | _ __    __ _   ___ | | __ ___     _   ___
   |'-_ _-'|'-_ _-'|  |  _ < | | | || || | / _` || '_ \  / _` | / __|| |/ // __|   | | / _ \
   |   |   |   |   |  | |_) || |_| || || || (_| || |_) || (_| || (__ |   < \__ \ _ | || (_) |
    '-_|_-' '-_|_-'   |____/  \__,_||_||_| \__,_|| .__/  \__,_| \___||_|\_\|___/(_)|_| \___/
                                                 | |
                                                 |_|
   
   :: Built with Spring Boot :: 2.1.3.RELEASE
   ``` 
    
1. kpack rebuilds with source code updates
    
   Push any update to the forked sample app repository configured in step #4. In a short amount of time, kpack should recognize the update and automatically rebuild your image.  
    
   You can see this happen by running `kubectl get builds`
   ```
   kubectl get builds
   ``` 
   You should see a new build with
   
   ```
   NAME                                IMAGE                                                                                           SUCCEEDED
   tutorial-image-build-1-8mqkc       gcr.io/project-name/app@sha256:6744b3b24a7ab8d2b45d7673313c180daccb534b3d931369d0aa9805712f34b   True
   tutorial-image-build-2-xsf2l                                                                                                        Unknown
   ```

   You can tail the logs for the image with log utility used in step #5
   
   ```
   logs -image tutorial-image -b 2  
   ```
    
1. kpack rebuilds with buildpack updates
    
    The next time the `cloudfoundry/cnb:bionic` is updated, kpack will detect if it contains buildpack updates to any of the buildpacks used by the tutorial image.
    If there is a buildpack update, kpack will automatically create a new build to rebuild your image.    
    