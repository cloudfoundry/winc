$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

Debug "$(gci env:* | sort-object name | Out-String)"
Configure-Groot "$env:WINC_TEST_ROOTFS"
Configure-Winc-Network delete

go run github.com/onsi/ginkgo/v2/ginkgo -p -r --race -keep-going --randomize-suites --fail-on-pending --flake-attempts 3 --skip-package winc-network,perf
if ($LastExitCode -ne 0) {
  throw "tests failed"
}

go run github.com/onsi/ginkgo/v2/ginkgo -r --race -keep-going --randomize-suites --fail-on-pending --flake-attempts 3 ./integration/perf ./integration/winc-network
if ($LastExitCode -ne 0) {
  throw "tests failed"
}
