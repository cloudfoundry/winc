$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

Debug "$(gci env:* | sort-object name | Out-String)"
Configure-Groot "$env:WINC_TEST_ROOTFS"
Configure-Winc-Network delete

Invoke-Expression "go run github.com/onsi/ginkgo/v2/ginkgo $args --flake-attempts 3 --skip-package winc-network,perf"
if ($LastExitCode -ne 0) {
  throw "tests failed"
}

Invoke-Expression "go run github.com/onsi/ginkgo/v2/ginkgo $args --flake-attempts 3 ./integration/perf ./integration/winc-network"
if ($LastExitCode -ne 0) {
  throw "tests failed"
}
