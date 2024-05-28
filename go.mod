module code.cloudfoundry.org/winc

go 1.21

toolchain go1.22.2

require (
	code.cloudfoundry.org/filelock v0.0.0-20240528151814-fec2f63a5995
	code.cloudfoundry.org/localip v0.0.0-20240528171257-80c5abae8ed6
	github.com/Microsoft/hcsshim v0.12.3
	github.com/blang/semver v3.5.1+incompatible
	github.com/hectane/go-acl v0.0.0-20230122075934-ca0b05cb1adb
	github.com/miekg/dns v1.1.59
	github.com/mitchellh/go-ps v1.0.0
	github.com/onsi/ginkgo/v2 v2.19.0
	github.com/onsi/gomega v1.33.1
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/opencontainers/runtime-tools v0.9.1-0.20221107090550-2e043c6bd626
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.3
	github.com/urfave/cli v1.22.15
	golang.org/x/sync v0.7.0
	golang.org/x/sys v0.20.0
	golang.org/x/text v0.15.0
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/containerd/cgroups v1.1.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/pprof v0.0.0-20240528025155-186aa0362fba // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/tools v0.21.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/Microsoft/hcsshim => github.com/Microsoft/hcsshim v0.9.10
	github.com/opencontainers/runtime-spec => github.com/opencontainers/runtime-spec v1.0.1-0.20171004210716-34170989dc28
	github.com/opencontainers/runtime-tools => github.com/opencontainers/runtime-tools v0.0.0-20170728063910-e29f3ca4eb80
)
