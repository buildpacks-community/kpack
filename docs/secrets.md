# Secrets

kpack utilizes kubernetes secrets to configure credentials to publish OCI images to docker registries and access private github repositories.

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

Host key checking is disabled by default, it can be enabled by setting the `INSECURE_SSH_TRUST_UNKNOWN_HOSTS` environment variable on the controller to `false`.

When host key checking is enabled, you can use the optional `known_hosts` field on the ssh auth secret. If it is not specified, the build will use the `SSH_KNOWN_HOSTS` environment variable before checking `~/.ssh/known_hosts` and `/etc/ssh/ssh_known_hosts`.
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-ssh-auth
  annotations:
    kpack.io/git: git@github.com
type: kubernetes.io/ssh-auth
stringData:
  known_hosts: <ssh-keyscan output>
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

### Blob Secrets

Secrets are used with a `kpack.io/blob` annotation that references a hostname for a blob location. Only one of username/password, bearer, or authorization is allowed.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: blob-secret
  annotations:
    kpack.io/blob: my-blob-store.com
stringData:
  username: <username>
  password: <password>

  bearer: <oauth2 token>

  authorization: <third-party-auth-header>
```

### Service Account

To use these secrets with kpack create a service account and reference the service account in image and build resources. When configuring the image resource, reference the `name` of your registry credential and the `name` of your git credential.

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
