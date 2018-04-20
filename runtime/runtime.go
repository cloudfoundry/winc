package runtime

import (
	"io"
	"os"

	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/runtime/container"
	"code.cloudfoundry.org/winc/runtime/winsyscall"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

//go:generate counterfeiter -o fakes/mounter.go --fake-name Mounter . Mounter
type Mounter interface {
	Mount(pid int, volumePath string) error
	Unmount(pid int) error
}

//go:generate counterfeiter -o fakes/state_factory.go --fake-name StateFactory . StateFactory
type StateFactory interface {
	NewManager(*logrus.Entry, *hcs.Client, *winsyscall.WinSyscall, string, string) StateManager
}

//go:generate counterfeiter -o fakes/state_manager.go --fake-name StateManager . StateManager
type StateManager interface {
	Initialize(string) error
	Delete() error
	SetFailure() error
	SetSuccess(hcs.Process) error
	State() (*specs.State, error)
}

//go:generate counterfeiter -o fakes/container_factory.go --fake-name ContainerFactory . ContainerFactory
type ContainerFactory interface {
	NewManager(*logrus.Entry, *hcs.Client, string) ContainerManager
}

//go:generate counterfeiter -o fakes/container_manager.go --fake-name ContainerManager . ContainerManager
type ContainerManager interface {
	Spec(string) (*specs.Spec, error)
	Create(*specs.Spec) error
	Exec(*specs.Process, bool) (hcs.Process, error)
	Stats() (container.Statistics, error)
	Delete(bool) error
}

//go:generate counterfeiter -o fakes/process_wrapper.go --fake-name ProcessWrapper . ProcessWrapper
type ProcessWrapper interface {
	Wrap(hcs.Process) WrappedProcess
}

//go:generate counterfeiter -o fakes/wrapped_process.go --fake-name WrappedProcess . WrappedProcess
type WrappedProcess interface {
	AttachIO(io.Reader, io.Writer, io.Writer) (int, error)
	SetInterrupt(chan os.Signal)
	WritePIDFile(string) error
}

//go:generate counterfeiter -o fakes/hcsquery.go --fake-name HCSQuery . HCSQuery
type HCSQuery interface {
	GetContainers(hcsshim.ComputeSystemQuery) ([]hcsshim.ContainerProperties, error)
}

type IO struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type Runtime struct {
	stateFactory     StateFactory
	containerFactory ContainerFactory
	mounter          Mounter
	hcsQuery         HCSQuery
	rootDir          string
}

func New(s StateFactory, c ContainerFactory, m Mounter, h HCSQuery, rootDir string) *Runtime {
	return &Runtime{
		stateFactory:     s,
		containerFactory: c,
		mounter:          m,
		hcsQuery:         h,
		rootDir:          rootDir,
	}
}

func (r *Runtime) Create(containerId, bundlePath string) error {
	client := hcs.Client{}
	cm := r.containerFactory.NewManager(nil, &client, containerId)

	wsc := winsyscall.WinSyscall{}
	sm := r.stateFactory.NewManager(nil, &client, &wsc, containerId, r.rootDir)

	spec, err := cm.Spec(bundlePath)
	if err != nil {
		return err
	}

	if err := cm.Create(spec); err != nil {
		return err
	}

	if err := sm.Initialize(bundlePath); err != nil {
		//		cm.Delete(true)
		return err
	}
	return nil
}

func (r *Runtime) Delete(containerId string, force bool) error {
	return nil
}

func (r *Runtime) Events(containerId string, output io.Writer, showStats bool) error {
	return nil
}

func (r *Runtime) Exec(containerId, processConfigFile, pidFile string, processOverrides *specs.Process, io IO, detach bool) error {
	return nil
}

func (r *Runtime) Run(containerId, bundlePath, pidFile string, io IO, detach bool) error {
	return nil
}

func (r *Runtime) Start(containerId, pidFile string) error {
	return nil
}

func (r *Runtime) State(containerId string) error {
	return nil
}
