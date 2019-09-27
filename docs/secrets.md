# Secrets

kpack utilizes kubernetes secrets to configure credentials to publish images to docker registries and access private github repositories.   

### Docker Registry Secrets

kubernetes.io/basic-auth secrets are used with a `build.pivotal.io/docker` annotation that references a docker registry.      

GCR example
  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: basic-docker-user-pass
    annotations:
      build.pivotal.io/docker: gcr.io
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
    build.pivotal.io/docker: index.docker.io
type: kubernetes.io/basic-auth
stringData:
  username: <username>
  password: <password>
```
        
> Note: The secret must be annotated with the registry prefix for its corresponding registry. For [dockerhub](https://hub.docker.com/) this should be `index.docker.io`. For GCR this should be `gcr.io`. 

### Git Registry Secrets

kubernetes.io/basic-auth secrets are used with a `build.pivotal.io/git` annotation that references a remote git location.      

For github, this would look like
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: basic-git-user-pass
  annotations:
    build.pivotal.io/git: https://github.com
type: kubernetes.io/basic-auth
stringData:
  username: <username>
  password: <password>
```

If your github account has 2 factor auth configured, create a personal access token using [this procedure](https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line).

Configure your secret for github like this:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: basic-git-user-pass
  annotations:
    build.pivotal.io/git: https://github.com
type: kubernetes.io/basic-auth
stringData:
  username: <generated-token>
  password: x-oauth-basic
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
  - name: basic-git-user-pass
```
