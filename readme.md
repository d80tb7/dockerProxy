# DockerProxy

DockerProxy is a proxy server that acts as an intermediary between clients and Docker registries, such as Artifactory. It provides a caching layer for metadata requests, reducing the load on the backend registry when the `ImagePullPolicy` is set to `Always`.

## Purpose

The primary purpose of DockerProxy is to optimize the retrieval of Docker image metadata by caching responses and serving them from the cache for subsequent requests. This is particularly useful when the `ImagePullPolicy` is set to `Always`, as it ensures that the proxy handles repeated requests without forwarding them to the backend registry each time.

## Features

- Caching of Docker image metadata requests
- Reverse proxy functionality for Docker registries
- Supports HTTP and HTTPS connections
- TLS certificate management
- Graceful shutdown on receiving SIGTERM signal

## Configuration

DockerProxy can be configured using a JSON or YAML configuration file. The configuration file allows you to customize various parameters, including:

- Server port and TLS settings
- Caching behavior and eviction period
- Read, write, and idle timeouts

The configuration file should be provided as a command-line argument using the `-config` flag. If no argument is supplied, the default configuration file path is `/etc/dockerProxy/config`.

## Usage

1. Build the DockerProxy application:

   ```shell
   go build -o dockerProxy main.go

2. Create a configuration file
```
{
   "ServerPort": "8080",
   "UseTLS": false,
   "CertFilePath": "/path/to/cert.pem",
   "KeyFilePath": "/path/to/key.pem",
   "ReadTimeout": "5m",
   "WriteTimeout": "5m",
   "IdleTimeout": "10m",
   "CacheEvictionPeriod": "10m"
}
```
3. Start the Docker Proxy  Server
```shell
./dockerProxy -config /path/to/config.json
```

4. Point your Docker clients to use DockerProxy as the registry endpoint. Adjust the ImagePullPolicy to Always to enable caching:
```
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
    - name: my-container
      image: my-registry/image:tag
      imagePullPolicy: Always
```

