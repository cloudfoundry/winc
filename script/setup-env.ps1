$env:WINC_TEST_ROOTFS="docker:///cloudfoundry/windows2016fs"

cd ..\groot-windows
go build -o groot.exe main.go
cd ..\winc
$env:GROOT_BINARY="$PWD\..\groot-windows\groot.exe"

$env:GROOT_IMAGE_STORE="C:\Windows\TEMP\test"
