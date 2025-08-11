> [!CAUTION]
> This repository has been in-lined (using git-subtree) into [winc-release](https://github.com/cloudfoundry/winc-release/pull/46). Please make any
> future contributions directly to winc-release.

![clippy](https://media.giphy.com/media/13V60VgE2ED7oc/giphy.gif)

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

`WINDOWS_VERSION` to your version of Windows.

`WINC_TEST_ROOTFS` to the path to a container image.

`GROOT_BINARY` to the path of the groot executable to use while running integration test.

`GROOT_IMAGE_STORE` to the path of the directory that groot uses for layers and the volume.

E.g.
```
$env:WINDOWS_VERSION="2019"
$env:WINC_TEST_ROOTFS="docker:///cloudfoundry/windows2016fs:2019"
$env:GROOT_BINARY="$env:GOPATH\bin\groot.exe"
$env:GROOT_IMAGE_STORE="C:\ProgramData\groot"
ginkgo -r integration/
```

To install [Ginkgo](https://onsi.github.io/ginkgo/
) and run the tests:
```
go get github.com/onsi/ginkgo/...
cd $GOPATH/src/code.cloudfoundry.org/winc
ginkgo -r -race -keepGoing
```

### Using

Check out [winc bosh release readme](https://github.com/cloudfoundry-incubator/winc-release/blob/develop/README.md) for creating new containers using winc.
