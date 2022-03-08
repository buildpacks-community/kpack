# kpack with Cosign Tutorial

This tutorial will walk through creating a kpack [image](image.md) resource to build a docker image from source that is signed with cosign. This tutorial builds upon the steps in the [kpack Tutorial](tutorial.md).

###  Prerequisites
1. kpack is installed and available on a Kubernetes cluster

    > Follow these docs to [install and setup kpack](install.md)

2. Cosign
    > Follow the official docs to [install cosign](https://docs.sigstore.dev/cosign/installation/)

3. A kpack Builder or ClusterBuilder and Image resource configured.

    > Make sure that you have followed and completed all the steps in the [kpack Tutorial](tutorial.md)

### Tutorial

1. Generate a cosign key pair

   The `cosign generate-key-pair` command generates a key-pair for you and stores it as a kubernetes secret with name `tutorial-cosign-key-pair` in the `default` namespace.

   ```bash
   cosign generate-key-pair k8s://default/tutorial-cosign-key-pair
   ```

   The command will ask you to enter a password for the private key. Enter any password you want. After the command has completed successfully, you should see the following output:

   ```
   Successfully created secret tutorial-cosign-key-pair in namespace default
   Public key written to cosign.pub
   ```

   You should see a `cosign.pub` file in your current folder. Keep this file as it will be needed to verify the signature of the images built in this tutorial.


   If you are using [dockerhub](https://hub.docker.com/) or a registry that does not support OCI media types, you need to add the annotation `kpack.io/cosign.docker-media-types: "1"` to the cosign secret. The secret `tutorial-cosign-key-pair` should look something like this:

   ```yaml
   apiVersion: v1
   kind: Secret
   type: Opaque
   metadata:
     name: tutorial-cosign-key-pair
     namespace: default
     annotations:
       kpack.io/cosign.docker-media-types: "1"
   data:
     cosign.key: <PRIVATE KEY DATA>
     cosign.password: <COSIGN PASSWORD>
     cosign.pub: <PUBLIC KEY DATA>
   ```

    > Note: Learn more about configuring cosign key pairs with the [kpack image documentation](image.md#cosign-configuration)


2. Create or modify the tutorial service account that is referenced in the Image resource so it includes the cosign key pair secret created in the previous step.

   Just by adding a cosign secret to the service account that is referenced in an Image resource enables cosign signing.

   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: tutorial-cosign-service-account
     namespace: default
   secrets:
   - name: tutorial-registry-credentials
   - name: tutorial-cosign-key-pair
   imagePullSecrets:
   - name: tutorial-registry-credentials
   ```

   Apply that service account to the cluster

   ```bash
   kubectl apply -f cosign-service-account.yaml
   ```

3. Create an Image resource:

   We will create a sample Image resource that builds with the same Builder from the [kpack Tutorial](tutorial.md). Note that this Image has a different name and references `tutorial-cosign-service-account`.

   ```yaml
   apiVersion: kpack.io/v1alpha2
   kind: Image
   metadata:
     name: tutorial-cosign-image
     namespace: default
   spec:
     tag: <DOCKER-IMAGE-TAG>
     serviceAccountName: tutorial-cosign-service-account
     builder:
       name: my-builder
       kind: Builder
     source:
       git:
         url: https://github.com/spring-projects/spring-petclinic
         revision: 82cb521d636b282340378d80a6307a08e3d4a4c4
   ```

   - Replace `<DOCKER-IMAGE-TAG>` with a valid image tag that exists in the registry you configured with the `--docker-server` flag when creating a Secret in step #1 of the [kpack Tutorial](tutorial.md). Something like: your-name/app or gcr.io/your-project/app

   Apply that Image resource to the cluster

   ```bash
   kubectl apply -f image-cosign.yaml
   ```

   Once the Image resource finishes building you can get the fully resolved built OCI image with `kubectl get`

   ```
   kubectl -n default get image tutorial-cosign-image
   ```

   The output should look something like this:
   ```
   NAME                  LATESTIMAGE                                        READY
   tutorial-cosign-image index.docker.io/your-project/app@sha256:6744b...   True
   ```

4. Verify image signature

   The image that was built in the previous step should have been signed. Here we use the `cosign.pub` public key file that was generated in the first step.
   ```bash
   cosign verify --key cosign.pub <latest-image-with-digest>
   ```

   You should see an output similar to this:
   ```
   Verification for <latest-image-with-digest> --
   The following checks were performed on each of these signatures:
    - The cosign claims were validated
    - The signatures were verified against the specified public key
    - Any certificates were verified against the Fulcio roots.
   ```