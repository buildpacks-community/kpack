# Build Service System

## Install

To create CRDs:
```bash
kubectl apply -f ./config/
```

## Test locally

Setup minikube
In case you have a minikube cluster already running, tear it down first.
```bash
minikube delete
```

Start a minikube cluster on Mac
```bash
minikube start --memory=4096 --cpus=4 --vm-driver=hyperkit --bootstrapper=kubeadm --insecure-registry "registry.default.svc.cluster.local:5000"
```

Start a minikube cluster on Linux
```bash
minikube start --memory=4096 --cpus=4 --vm-driver=kvm2 --bootstrapper=kubeadm --insecure-registry "registry.default.svc.cluster.local:5000"
sudo ifconfig lo:0 192.168.64.1
```

Update `/etc/hosts` by adding the name registry.default.svc.cluster.local on the same line as the entry for localhost. It should look something like this:
```bash
##
127.0.0.1       localhost registry.default.svc.cluster.local
255.255.255.255 broadcasthost
::1             localhost
```

Update the minikube `/etc/hosts` with the host ip for registry.default.svc.cluster.local
 ```bash
minikube ssh \
"echo \"192.168.64.1       registry.default.svc.cluster.local\" \
| sudo tee -a  /etc/hosts"
```

To execute the tests:
```bash
go test -v ./...
```