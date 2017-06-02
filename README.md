# winc

## Introduction

`winc` is a CLI tool for spawning and running containers on Windows according to the OCI specification.

## Building

* Install [Golang 1.8](https://golang.org/dl/)
  * Make sure you have set a `GOPATH`
* Install [git](https://git-for-windows.github.io/)
* [Install mingw-w64](https://sourceforge.net/projects/mingw-w64/)
  * Select `x86_64` as the target architecture
  * After install, ensure that `gcc.exe` is in your `PATH`
* Open a new `powershell` instance
* `go get code.cloudfoundry.org/winc/...`
  * `winc.exe` will be in `$env:GOBIN` or `$env:GOPATH\bin`

## Testing

* Install [Ginkgo](https://onsi.github.io/ginkgo/)
  * `go get github.com/onsi/ginkgo/...`
* Set the `WINC_TEST_ROOTFS` environment variable to the path to a container image
  * e.g. in `powershell` to test with the `microsoft/windowsservercore` Docker image: `$env:WINC_TEST_ROOTFS = (docker inspect microsoft/windowsservercore | ConvertFrom-Json).GraphDriver.Data.Dir`
* `cd $GOPATH/src/code.cloudfoundry.org/winc`
* `ginkgo -r -race -keepGoing`
