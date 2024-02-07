module code.cloudfoundry.org/winc

go 1.19

require (
	code.cloudfoundry.org/filelock v0.0.0-20240116200225-91558a7180a4
	code.cloudfoundry.org/localip v0.0.0-20231206170541-8d74d8ef21ec
	github.com/Microsoft/hcsshim v0.11.4
	github.com/blang/semver v3.5.1+incompatible
	github.com/hectane/go-acl v0.0.0-20230122075934-ca0b05cb1adb
	github.com/miekg/dns v1.1.58
	github.com/mitchellh/go-ps v1.0.0
	github.com/onsi/ginkgo/v2 v2.15.0
	github.com/onsi/gomega v1.31.1
	github.com/opencontainers/runtime-spec v1.1.0
	github.com/opencontainers/runtime-tools v0.9.1-0.20221107090550-2e043c6bd626
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.3
	github.com/urfave/cli v1.22.14
	golang.org/x/sync v0.6.0
	golang.org/x/sys v0.17.0
	golang.org/x/text v0.14.0
)

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/containerd/cgroups v1.1.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.3 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/pprof v0.0.0-20240207164012-fb44976bdcd5 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/mod v0.15.0 // indirect
	golang.org/x/net v0.20.0 // indirect
	golang.org/x/tools v0.17.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/Microsoft/hcsshim => github.com/Microsoft/hcsshim v0.9.10
	github.com/opencontainers/runtime-spec => github.com/opencontainers/runtime-spec v1.0.1-0.20171004210716-34170989dc28
	github.com/opencontainers/runtime-tools => github.com/opencontainers/runtime-tools v0.0.0-20170728063910-e29f3ca4eb80
)
