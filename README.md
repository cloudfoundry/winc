# winc

`winc` is a CLI tool for spawning and running containers on Windows according to the OCI specification.

### Building

#### Requirements

* [Golang](https://golang.org/dl/)
  * Make sure you have set a `GOPATH`
* [Git](https://git-for-windows.github.io/)
* [mingw-w64](https://sourceforge.net/projects/mingw-w64/)
  * Select `x86_64` as the target architecture
  * After install, ensure that `gcc.exe` is in your `PATH`

To clone and build `winc.exe`:

```
go get -d code.cloudfoundry.org/winc/...
cd $GOPATH/src/code.cloudfoundry.org/winc
go build ./cmd/winc
```

### Testing

Set the following environment variables first:

`WINDOWS_VERSION` to your version of Windows (e.g. `"2019"`).

`WINC_TEST_ROOTFS` to the path to a container image (e.g. to test with the `microsoft/windowsservercore` Docker image - `"docker:///microsoft/windowsservercore"`).

`GROOT_BINARY` to the path of the groot executable to use while running integration test.

`GROOT_IMAGE_STORE` to the path of the directory that groot uses for layers and the volume (e.g. `"C:\ProgramData\groot"`).

To install [Ginkgo](https://onsi.github.io/ginkgo/
) and run the tests:
```
go get github.com/onsi/ginkgo/...
cd $GOPATH/src/code.cloudfoundry.org/winc
ginkgo -r -race -keepGoing
```

### Using

Check out [winc bosh release readme](https://github.com/cloudfoundry-incubator/winc-release/blob/develop/README.md) for creating new containers using winc.
