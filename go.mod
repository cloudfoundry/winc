module code.cloudfoundry.org/winc

go 1.19

require (
	code.cloudfoundry.org/credhub-cli v0.0.0-20230731130338-2495d5a758cb
	code.cloudfoundry.org/filelock v0.0.0-20230612152934-de193be258e4
	code.cloudfoundry.org/localip v0.0.0-20230612151424-f52ecafaffc4
	github.com/Microsoft/hcsshim v0.9.9
	github.com/PaesslerAG/jsonpath v0.1.1
	github.com/blang/semver v3.5.1+incompatible
	github.com/hectane/go-acl v0.0.0-20190112205748-6937c4c474eb
	github.com/miekg/dns v1.1.54
	github.com/mitchellh/go-ps v0.0.0-20170309133038-4fdf99ab2936
	github.com/onsi/ginkgo/v2 v2.11.0
	github.com/onsi/gomega v1.27.10
	github.com/opencontainers/runtime-spec v1.1.0-rc.3
	github.com/opencontainers/runtime-tools v0.9.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.3
	github.com/urfave/cli v1.22.14
	golang.org/x/sync v0.3.0
	golang.org/x/sys v0.10.0
	golang.org/x/text v0.11.0
)

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/PaesslerAG/gval v1.0.0 // indirect
	github.com/cloudfoundry/go-socks5 v0.0.0-20180221174514-54f73bdb8a8e // indirect
	github.com/cloudfoundry/socks5-proxy v0.2.94 // indirect
	github.com/containerd/cgroups v1.1.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/pprof v0.0.0-20230705174524-200ffdc848b8 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/crypto v0.11.0 // indirect
	golang.org/x/mod v0.12.0 // indirect
	golang.org/x/net v0.12.0 // indirect
	golang.org/x/tools v0.11.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/opencontainers/runtime-spec => github.com/opencontainers/runtime-spec v1.0.1-0.20171004210716-34170989dc28
	github.com/opencontainers/runtime-tools => github.com/opencontainers/runtime-tools v0.0.0-20170728063910-e29f3ca4eb80
)
