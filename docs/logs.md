# kpack logs

Tailing the build logs is possible with the kpack log utility. 

### Install

Downloading the log utility for you operating system from the most recent [github release](https://github.com/pivotal/kpack/releases).

### Usage

To tail logs from all builds for an image  
```bash
logs -image <image-name> 
```

To tail logs from a specific build on an image  
```bash
logs -image <image-name> -build <build-number>
```

To tail logs from an image in a different namespace  
```bash
logs -image <image-name> -namespace <namespace>
```

> Note: The log utility will not exit when the build finishes.  
