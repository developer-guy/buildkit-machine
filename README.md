# buildkit-machine

buildkit-machine allows you to make buildkitd daemon accessible in your macOS environment. To do so, it uses [lima](https://github.com/lima-vm/lima), which is a Linux subsystem for macOS, under the hood. lima spins up a VM that runs buildkitd daemon in a rootless way which means that sock file of the buildkitd daemon is now be able to accessible from `/run/user/502/buildkit/buildkitd`.

## Overview

![gif](./res/anim.gif)

## Installation

```shell
$ go install github.com/developer-guy/buildkit-machine@latest
```

## Usage

> Please ensure you've installed limactl because buildkit-machine will use limactl executable under the hood.

To make it accessible Buildkitd Daemon over socket:

```shell
$ buildkit-machine start buildkitd --unix $(pwd)/buildkitd.sock
```

To make it accessible Buildkitd Daemon over TCP connection:

```shell
$ buildkit-machine start builtkitd --tcp 9999
```

Once you make buildkitd accessible to your host, you can be able to use client tooling such as `buildctl` to start building and pushing container images. There is an on-going issue in [Docker Buildx](https://github.com/docker/buildx/issues/23) side to let Buildx to connect remote Buildkit daemon. Once it is ready, we can use buildx too.
