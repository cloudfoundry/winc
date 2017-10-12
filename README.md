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
./scripts/build.ps1
```

### Testing

Set the `WINC_TEST_ROOTFS` environment variable to the path to a container image, e.g. in `powershell` to test with the `microsoft/windowsservercore` Docker image:

`$env:WINC_TEST_ROOTFS = (docker inspect microsoft/windowsservercore | ConvertFrom-Json).GraphDriver.Data.Dir`

To install [Ginkgo](https://onsi.github.io/ginkgo/
) and run the tests:
```
go get github.com/onsi/ginkgo/...
cd $GOPATH/src/code.cloudfoundry.org/winc
ginkgo -r -race -keepGoing
```

### Using

The following `powershell` script can be used to quickly create a new container. It takes an optional container ID as an argument. It requires `winc.exe` and `winc-image.exe` to be on your path, and `quota.dll` to be in the same directory as `winc-image.exe`. $env:WINC_TEST_ROOTFS must be set.

```
if (!(Get-Command "winc.exe" -ErrorAction SilentlyContinue)) {
   Write-Host "Unable to find winc.exe"
   Exit 1
}

if (!(Get-Command "winc-image.exe" -ErrorAction SilentlyContinue)) {
   Write-Host "Unable to find winc-image.exe"
   Exit 1
}

$wincImageParent = Split-Path (Get-Command winc-image.exe).Path
$quotaDllPath = Join-Path "$wincImageParent" "quota.dll"
if (!(Test-Path $quotaDllPath)) {
   Write-Host "Unable to find quota.dll in the same directory as winc-image.exe"
   Exit 1
}

$containerId = $args[0]
if (!$containerId) {
  $containerId = [guid]::NewGuid()
}

$rootfs = $env:WINC_TEST_ROOTFS 

$config = winc-image.exe create $rootfs $containerId | ConvertFrom-Json
$config.ociVersion = "1.0.0-rc6"
$config.PSObject.Properties.Remove("rootfs")

$containerDir = Join-Path $env:TEMP $containerId
$configPath = Join-Path $containerDir "config.json"
rm -Recurse -Force -ErrorAction SilentlyContinue $containerDir
mkdir $containerDir | Out-Null
Set-Content -Path $configPath -Value ($config | ConvertTo-Json)

winc.exe --root "C:\var\lib\winc-image" create -b $containerDir $containerId

Write-Host "Created container $containerId"
```
