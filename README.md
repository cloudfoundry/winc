# winc

## Introduction

`winc` is a CLI tool for spawning and running containers on Windows according to the OCI specification.

## Compilation

* Install [`Golang 1.8`](https://golang.org/dl/)
  * Make sure you have set a `GOPATH`
* Install [`git`](https://git-for-windows.github.io/)
* [Install `mingw-w64`](https://sourceforge.net/projects/mingw-w64/)
  * Select `x86_64` as the target architecture
  * After install, ensure that `gcc.exe` is in your `PATH`
* Open a new `powershell` instance
* `go get code.cloudfoundry.org/winc/...`
* `winc.exe` will be in `$env:GOBIN` or `$env:GOPATH\bin`
