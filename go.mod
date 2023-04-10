module code.cloudfoundry.org/winc

go 1.19

require (
	code.cloudfoundry.org/filelock v0.0.0-20230410204127-470838d066c5
	code.cloudfoundry.org/localip v0.0.0-20170223024724-b88ad0dea95c
	github.com/Microsoft/hcsshim v0.8.7
	github.com/blang/semver v3.5.1+incompatible
	github.com/hectane/go-acl v0.0.0-20190112205748-6937c4c474eb
	github.com/miekg/dns v1.1.3
	github.com/mitchellh/go-ps v0.0.0-20170309133038-4fdf99ab2936
	github.com/onsi/ginkgo/v2 v2.9.2
	github.com/onsi/gomega v1.27.6
	github.com/opencontainers/runtime-spec v1.0.1-0.20171004210716-34170989dc28
	github.com/opencontainers/runtime-tools v0.0.0-20181011054405-1d69bd0f9c39
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/urfave/cli v1.20.0
	golang.org/x/sync v0.1.0
	golang.org/x/sys v0.7.0
	golang.org/x/text v0.9.0
)

require (
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/containerd/cgroups v0.0.0-20190919134610-bf292b21730f // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/pprof v0.0.0-20230406165453-00490a63f317 // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	go.opencensus.io v0.22.0 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/tools v0.8.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/opencontainers/runtime-tools => github.com/opencontainers/runtime-tools v0.0.0-20170728063910-e29f3ca4eb80
