# Secrets

kpack utilizes kubernetes secrets to configure credentials to publish images to docker registries and access private github repositories.

Corresponding `kp` cli command docs [here](https://github.com/vmware-tanzu/kpack-cli/blob/main/docs/kp_secret.md).

### Docker Registry Secrets

kubernetes.io/basic-auth secrets are used with a `kpack.io/docker` annotation that references a docker registry.      

GCR example
  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: basic-docker-user-pass
    annotations:
      kpack.io/docker: gcr.io
  type: kubernetes.io/basic-auth
  stringData:
    username: <username>
    password: <password>
  ```

Docker Hub example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: basic-docker-user-pass
  annotations:
    kpack.io/docker: https://index.docker.io/v1/
type: kubernetes.io/basic-auth
stringData:
  username: <username>
  password: <password>
```
        
> Note: The secret must be annotated with the registry prefix for its corresponding registry. For [dockerhub](https://hub.docker.com/) this should be `https://index.docker.io/v1/`. For GCR this should be `gcr.io`. 

Additionally, both `kubernetes.io/dockerconfigjson` and `kubernetes.io/dockercfg` type secrets are supported as credentials to write to docker registries.
These credentials do not need to be annotated with the registry.

Docker Config Json example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: docker-configjson
type: kubernetes.io/dockerconfigjson
stringData:
  .dockerconfigjson: <contents of .docker/config.json>
```

Docker Cfg example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: docker-cfg
type: kubernetes.io/dockercfg
stringData:
  .dockercfg: <contents of .dockercfg>
```

### Git Secrets

kubernetes.io/basic-auth secrets are used with a `kpack.io/git` annotation that references a remote git location.      

For github, the basic auth secret would look like
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: basic-git-user-pass
  annotations:
    kpack.io/git: https://github.com
type: kubernetes.io/basic-auth
stringData:
  username: <username>
  password: <password>
```

For github, the ssh auth secret would look like
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-ssh-auth
  annotations:
    kpack.io/git: git@github.com
type: kubernetes.io/ssh-auth
stringData:
  ssh-privatekey: <x509-private-key>
```

If your github account has 2 factor auth configured, create a personal access token using [this procedure](https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line).

Configure your secret for github like this:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: basic-git-user-pass
  annotations:
    kpack.io/git: https://github.com
type: kubernetes.io/basic-auth
stringData:
  username: <your-username>
  password: <generated-token>
```

### Service Account

To use these secrets with kpack create a service account and reference the service account in image and build config. When configuring the image resource, reference the `name` of your registry credential and the `name` of your git credential.   

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: service-account
secrets:
  - name: basic-docker-user-pass
  - name: docker-configjson
  - name: docker-cfg
  - name: basic-git-user-pass
  - name: git-ssh-auth
```
