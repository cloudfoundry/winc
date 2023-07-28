$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

Debug "$(gci env:* | sort-object name | Out-String)"
Configure-Groot "$env:WINC_TEST_ROOTFS"
Configure-Winc-Network delete

ginkgo.exe -p -r --race -keep-going --randomize-suites --fail-on-pending --skip-package winc-network,perf --flake-attempts 3
if ($LastExitCode -ne 0) {
  throw "tests failed"
}

ginkgo.exe -r --race -keep-going --randomize-suites --fail-on-pending --flake-attempts 3 ./integration/perf ./integration/winc-network
if ($LastExitCode -ne 0) {
  throw "tests failed"
}
