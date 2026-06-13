---
title: "Installation"
description: "Install cs229 from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/cs229-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `cs229` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/cs229-cli/cmd/cs229@latest
```

That puts `cs229` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/cs229-cli
cd cs229-cli
make build        # produces ./bin/cs229
./bin/cs229 version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/cs229:latest --help
```

## Checking the install

```bash
cs229 version
```

prints the version and exits.
