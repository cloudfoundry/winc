$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

Debug "$(gci env:* | sort-object name | Out-String)"
Configure-Groot "$env:WINC_TEST_ROOTFS"
Configure-Winc-Network delete

ginkgo.exe -p -r --race -keep-going --randomize-suites --fail-on-pending --skip-package winc-network,perf
if ($LastExitCode -ne 0) {
  Exit 1
}

ginkgo.exe -r --race -keep-going --randomize-suites --fail-on-pending ./integration/perf ./integration/winc-network
if ($LastExitCode -ne 0) {
  Exit 1
}
