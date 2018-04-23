package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/runtime/config"
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
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Runtime struct {
	stateFactory     StateFactory
	containerFactory ContainerFactory
	mounter          Mounter
	hcsQuery         HCSQuery
	processWrapper   ProcessWrapper
	rootDir          string
}

func New(s StateFactory, c ContainerFactory, m Mounter, h HCSQuery, p ProcessWrapper, rootDir string) *Runtime {
	return &Runtime{
		stateFactory:     s,
		containerFactory: c,
		mounter:          m,
		hcsQuery:         h,
		processWrapper:   p,
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
		cm.Delete(false)
		return err
	}
	return nil
}

func (r *Runtime) Delete(containerId string, force bool) error {
	client := hcs.Client{}
	cm := r.containerFactory.NewManager(nil, &client, containerId)

	wsc := winsyscall.WinSyscall{}
	sm := r.stateFactory.NewManager(nil, &client, &wsc, containerId, r.rootDir)

	var errs []string

	ociState, err := sm.State()
	if err != nil {
		//logger.Error(err)

		if _, ok := err.(*hcs.NotFoundError); ok {
			if force {
				return nil
			}
			return err
		}

		errs = append(errs, err.Error())
	} else if ociState.Pid != 0 {
		if err := r.mounter.Unmount(ociState.Pid); err != nil {
			//	logger.Error(err)
			errs = append(errs, err.Error())
		}
	}

	if err := sm.Delete(); err != nil {
		//logger.Error(err)
		errs = append(errs, err.Error())
	}

	if err := cm.Delete(force); err != nil {
		//logger.Error(err)
		errs = append(errs, err.Error())
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func (r *Runtime) Events(containerId string, output io.Writer, showStats bool) error {
	client := hcs.Client{}
	cm := r.containerFactory.NewManager(nil, &client, containerId)

	stats, err := cm.Stats()
	if err != nil {
		return err
	}

	if showStats {
		if output == nil {
			return errors.New("provided output is nil")
		}
		statsJson, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return err
		}

		_, err = output.Write(statsJson)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Runtime) Exec(containerId, processConfigFile, pidFile string, processOverrides *specs.Process, io IO, detach bool) (int, error) {
	client := hcs.Client{}
	cm := r.containerFactory.NewManager(nil, &client, containerId)

	// TODO: real logging
	logrus.SetOutput(ioutil.Discard)
	logger := logrus.WithFields(logrus.Fields{
		"pidFile": pidFile,
		"detach":  detach,
	})
	processSpec, err := config.ValidateProcess(logger, processConfigFile, processOverrides)
	if err != nil {
		return 1, err
	}

	p, err := cm.Exec(processSpec, !detach)
	if err != nil {
		return 1, err
	}
	defer p.Close()

	wrappedProcess := r.processWrapper.Wrap(p)
	if err := wrappedProcess.WritePIDFile(pidFile); err != nil {
		return 1, err
	}

	if !detach {
		s := make(chan os.Signal, 1)
		wrappedProcess.SetInterrupt(s)
		return wrappedProcess.AttachIO(io.Stdin, io.Stdout, io.Stderr)
	}

	return 0, nil
}

func (r *Runtime) Run(containerId, bundlePath, pidFile string, io IO, detach bool) (int, error) {
	client := hcs.Client{}
	cm := r.containerFactory.NewManager(nil, &client, containerId)

	wsc := winsyscall.WinSyscall{}
	sm := r.stateFactory.NewManager(nil, &client, &wsc, containerId, r.rootDir)

	spec, err := cm.Spec(bundlePath)
	if err != nil {
		return 1, err
	}

	if err := cm.Create(spec); err != nil {
		return 1, err
	}

	if err := sm.Initialize(bundlePath); err != nil {
		cm.Delete(false)
		return 1, err
	}

	process, err := cm.Exec(spec.Process, !detach)
	if err != nil {
		if cErr, ok := errors.Cause(err).(*container.CouldNotCreateProcessError); ok {
			if sErr := sm.SetFailure(); sErr != nil {
				//logger.Error(sErr)
				//TODO: fixme
			}
			return 1, cErr
		}
		return 1, err
	}
	defer process.Close()

	if err := sm.SetSuccess(process); err != nil {
		return 1, err
	}

	if err := r.mounter.Mount(process.Pid(), spec.Root.Path); err != nil {
		return 1, err
	}

	wrappedProcess := r.processWrapper.Wrap(process)
	if err := wrappedProcess.WritePIDFile(pidFile); err != nil {
		return 1, err
	}

	if !detach {
		s := make(chan os.Signal, 1)
		wrappedProcess.SetInterrupt(s)

		exitCode, attachErr := wrappedProcess.AttachIO(io.Stdin, io.Stdout, io.Stderr)

		var errs []string

		ociState, err := sm.State()
		if err != nil {
			//logger.Error(err)
			errs = append(errs, err.Error())
		} else if ociState.Pid != 0 {
			//logger.Info(something useful)
			if err := r.mounter.Unmount(ociState.Pid); err != nil {
				//logger.Error(err)
				errs = append(errs, err.Error())
			}
		}

		if err := sm.Delete(); err != nil {
			//logger.Error(err)
			errs = append(errs, err.Error())
		}

		if err := cm.Delete(false); err != nil {
			//logger.Error(err)
			errs = append(errs, err.Error())
		}

		if attachErr != nil {
			return exitCode, attachErr
		}

		if len(errs) != 0 {
			return 1, errors.New(strings.Join(errs, "\n"))
		}

		return exitCode, nil
	}

	return 0, nil
}

func (r *Runtime) Start(containerId, pidFile string) error {
	client := hcs.Client{}
	cm := r.containerFactory.NewManager(nil, &client, containerId)

	wsc := winsyscall.WinSyscall{}
	sm := r.stateFactory.NewManager(nil, &client, &wsc, containerId, r.rootDir)

	ociState, err := sm.State()
	if err != nil {
		return err
	}

	if ociState.Status != "created" {
		return fmt.Errorf("cannot start a container in the %s state", ociState.Status)
	}

	spec, err := cm.Spec(ociState.Bundle)
	if err != nil {
		return err
	}

	process, err := cm.Exec(spec.Process, false)
	if err != nil {
		if cErr, ok := errors.Cause(err).(*container.CouldNotCreateProcessError); ok {
			if sErr := sm.SetFailure(); sErr != nil {
				//logger.Error(sErr) TODO: implement me
			}
			return cErr
		}
		return err
	}
	defer process.Close()

	if err := sm.SetSuccess(process); err != nil {
		return err
	}

	if err := r.mounter.Mount(process.Pid(), spec.Root.Path); err != nil {
		return err
	}

	wrappedProcess := r.processWrapper.Wrap(process)
	return wrappedProcess.WritePIDFile(pidFile)
}

func (r *Runtime) State(containerId string, output io.Writer) error {
	client := hcs.Client{}
	wsc := winsyscall.WinSyscall{}
	sm := r.stateFactory.NewManager(nil, &client, &wsc, containerId, r.rootDir)

	if output == nil {
		return errors.New("provided output is nil")
	}

	state, err := sm.State()
	if err != nil {
		return err
	}

	stateJson, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	_, err = output.Write(stateJson)
	return err
}
