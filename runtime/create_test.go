package runtime_test

import (
	"errors"

	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/runtime"
	"code.cloudfoundry.org/winc/runtime/fakes"
	"code.cloudfoundry.org/winc/runtime/winsyscall"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Create", func() {
	const (
		bundlePath  = "some/dir"
		rootDir     = "dir-for-state-and-things"
		containerId = "container-to-create"
	)
	var (
		mounter          *fakes.Mounter
		stateFactory     *fakes.StateFactory
		sm               *fakes.StateManager
		containerFactory *fakes.ContainerFactory
		cm               *fakes.ContainerManager
		processWrapper   *fakes.ProcessWrapper
		hcsQuery         *fakes.HCSQuery
		r                *runtime.Runtime
		spec             *specs.Spec

		credentialSpecPath string
	)

	BeforeEach(func() {
		mounter = &fakes.Mounter{}
		hcsQuery = &fakes.HCSQuery{}
		stateFactory = &fakes.StateFactory{}
		sm = &fakes.StateManager{}
		containerFactory = &fakes.ContainerFactory{}
		cm = &fakes.ContainerManager{}
		processWrapper = &fakes.ProcessWrapper{}
		spec = &specs.Spec{}

		stateFactory.NewManagerReturns(sm)
		containerFactory.NewManagerReturns(cm)

		cm.SpecReturns(spec, nil)
		cm.CredentialSpecFromFileReturns("", nil)

		config := runtime.Config{}
		r = runtime.New(stateFactory, containerFactory, mounter, hcsQuery, processWrapper, rootDir, credentialSpecPath, config)
	})

	It("loads the spec, creates the container, and intializes the state", func() {
		Expect(r.Create(containerId, bundlePath)).To(Succeed())

		_, c, id := containerFactory.NewManagerArgsForCall(0)
		Expect(*c).To(Equal(hcs.Client{}))
		Expect(id).To(Equal(containerId))

		_, c, wc, id, rd := stateFactory.NewManagerArgsForCall(0)
		Expect(*c).To(Equal(hcs.Client{}))
		Expect(*wc).To(Equal(winsyscall.WinSyscall{}))
		Expect(id).To(Equal(containerId))
		Expect(rd).To(Equal(rootDir))

		Expect(cm.SpecArgsForCall(0)).To(Equal(bundlePath))

		s, cs := cm.CreateArgsForCall(0)
		Expect(s).To(Equal(spec))
		Expect(cs).To(Equal(""))

		Expect(sm.InitializeArgsForCall(0)).To(Equal(bundlePath))
	})

	Context("when a non-empty credential spec env and filepath is provided", func() {
		BeforeEach(func() {
			credentialSpecPath = "/path/to/somewhere"
			config := runtime.Config{
				UaaCredhubClientId:     "hello",
				UaaCredhubClientSecret: "world",
				CredhubEndpoint:        "http://somewhere",
				CredhubCaCertificate:   "cert-value",
			}
			r = runtime.New(stateFactory, containerFactory, mounter, hcsQuery, processWrapper, rootDir, credentialSpecPath, config)

			cm.CredentialSpecFromEnvStub = func(envs []string, endpoint string, clientId string, clientSecret string, caCert string) (string, error) {
				Expect(clientId).To(Equal("hello"))
				Expect(clientSecret).To(Equal("world"))
				Expect(endpoint).To(Equal("http://somewhere"))
				Expect(caCert).To(Equal("cert-value"))
				return "credential-spec-contents", nil
			}
		})

		It("prefers loading credentials from env", func() {
			Expect(r.Create(containerId, bundlePath)).To(Succeed())

			_, c, id := containerFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(id).To(Equal(containerId))

			_, c, wc, id, rd := stateFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(*wc).To(Equal(winsyscall.WinSyscall{}))
			Expect(id).To(Equal(containerId))
			Expect(rd).To(Equal(rootDir))

			Expect(cm.SpecArgsForCall(0)).To(Equal(bundlePath))

			s, cs := cm.CreateArgsForCall(0)
			Expect(s).To(Equal(spec))
			Expect(cs).To(Equal("credential-spec-contents"))

			Expect(sm.InitializeArgsForCall(0)).To(Equal(bundlePath))

			Expect(cm.CredentialSpecFromFileCallCount()).To(Equal(0))
			Expect(cm.CredentialSpecFromEnvCallCount()).To(Equal(1))
		})
	})

	Context("when a non-empty credential spec env is provided", func() {
		BeforeEach(func() {
			credentialSpecPath = ""
			config := runtime.Config{
				UaaCredhubClientId:     "hello",
				UaaCredhubClientSecret: "world",
				CredhubEndpoint:        "http://somewhere",
				CredhubCaCertificate:   "cert-value",
			}
			r = runtime.New(stateFactory, containerFactory, mounter, hcsQuery, processWrapper, rootDir, credentialSpecPath, config)

			cm.CredentialSpecFromEnvStub = func(envs []string, endpoint string, clientId string, clientSecret string, caCert string) (string, error) {
				Expect(clientId).To(Equal("hello"))
				Expect(clientSecret).To(Equal("world"))
				Expect(endpoint).To(Equal("http://somewhere"))
				Expect(caCert).To(Equal("cert-value"))
				return "credential-spec-contents", nil
			}
		})

		It("loads the spec, creates the container, and intializes the state", func() {
			Expect(r.Create(containerId, bundlePath)).To(Succeed())

			_, c, id := containerFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(id).To(Equal(containerId))

			_, c, wc, id, rd := stateFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(*wc).To(Equal(winsyscall.WinSyscall{}))
			Expect(id).To(Equal(containerId))
			Expect(rd).To(Equal(rootDir))

			Expect(cm.SpecArgsForCall(0)).To(Equal(bundlePath))

			s, cs := cm.CreateArgsForCall(0)
			Expect(s).To(Equal(spec))
			Expect(cs).To(Equal("credential-spec-contents"))

			Expect(sm.InitializeArgsForCall(0)).To(Equal(bundlePath))
			Expect(cm.CredentialSpecFromEnvCallCount()).To(Equal(1))
		})

		Context("loading the credential spec fails", func() {
			BeforeEach(func() {
				cm.CredentialSpecFromEnvReturns("", errors.New("bad credential spec"))
			})

			It("returns the error", func() {
				err := r.Create(containerId, bundlePath)
				Expect(err).To(MatchError("bad credential spec"))
			})
		})
	})

	Context("when a non-empty credential spec filepath is provided", func() {
		BeforeEach(func() {
			credentialSpecPath = "/path/to/credential/spec"
			config := runtime.Config{}
			r = runtime.New(stateFactory, containerFactory, mounter, hcsQuery, processWrapper, rootDir, credentialSpecPath, config)

			cm.CredentialSpecFromFileStub = func(path string) (string, error) {
				Expect(path).To(Equal(credentialSpecPath))

				return "credential-spec-contents", nil
			}
		})

		It("loads the spec, creates the container, and intializes the state", func() {
			Expect(r.Create(containerId, bundlePath)).To(Succeed())

			_, c, id := containerFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(id).To(Equal(containerId))

			_, c, wc, id, rd := stateFactory.NewManagerArgsForCall(0)
			Expect(*c).To(Equal(hcs.Client{}))
			Expect(*wc).To(Equal(winsyscall.WinSyscall{}))
			Expect(id).To(Equal(containerId))
			Expect(rd).To(Equal(rootDir))

			Expect(cm.SpecArgsForCall(0)).To(Equal(bundlePath))

			s, cs := cm.CreateArgsForCall(0)
			Expect(s).To(Equal(spec))
			Expect(cs).To(Equal("credential-spec-contents"))

			Expect(sm.InitializeArgsForCall(0)).To(Equal(bundlePath))
		})

		Context("loading the credential spec fails", func() {
			BeforeEach(func() {
				cm.CredentialSpecFromFileReturns("", errors.New("bad credential spec"))
			})

			It("returns the error", func() {
				err := r.Create(containerId, bundlePath)
				Expect(err).To(MatchError("bad credential spec"))
			})
		})
	})

	Context("loading the spec fails", func() {
		BeforeEach(func() {
			cm.SpecReturns(nil, errors.New("bad spec"))
		})

		It("returns the error", func() {
			err := r.Create(containerId, bundlePath)
			Expect(err).To(MatchError("bad spec"))
		})
	})

	Context("creating the container fails", func() {
		BeforeEach(func() {
			cm.CreateReturns(errors.New("hcsshim fell over"))
		})

		It("returns the error", func() {
			err := r.Create(containerId, bundlePath)
			Expect(err).To(MatchError("hcsshim fell over"))
		})
	})

	Context("initializing state fails", func() {
		BeforeEach(func() {
			sm.InitializeReturns(errors.New("state init failed"))
		})

		It("deletes the container", func() {
			err := r.Create(containerId, bundlePath)
			Expect(err).To(MatchError("state init failed"))

			Expect(cm.DeleteCallCount()).To(Equal(1))
			force := cm.DeleteArgsForCall(0)
			Expect(force).To(Equal(false))
		})
	})
})
