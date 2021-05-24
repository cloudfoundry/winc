module code.cloudfoundry.org/winc

go 1.15

require (
	code.cloudfoundry.org/filelock v0.0.0-20180314203404-13cd41364639
	code.cloudfoundry.org/localip v0.0.0-20170223024724-b88ad0dea95c
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/Microsoft/hcsshim v0.8.17
	github.com/blang/semver v3.5.1+incompatible
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hectane/go-acl v0.0.0-20190112205748-6937c4c474eb
	github.com/miekg/dns v1.1.3
	github.com/mitchellh/go-ps v0.0.0-20170309133038-4fdf99ab2936
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.3
	github.com/opencontainers/runtime-spec v1.0.3-0.20200929063507-e6143ca7d51d
	github.com/opencontainers/runtime-tools v0.0.0-20181011054405-1d69bd0f9c39
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/urfave/cli v1.22.2
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/sys v0.0.0-20210324051608-47abb6519492
	golang.org/x/text v0.3.4
)

replace github.com/Microsoft/hcsshim v0.8.17 => github.com/greenhouse-org/hcsshim v0.6.8-0.20190130155644-d3cfe7c848cd
