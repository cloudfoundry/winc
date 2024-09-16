package runtime

import (
	"encoding/json"
	"fmt"
	"io"
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
	Mount(pid int, volumePath string, logger *logrus.Entry) error
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
	CredentialSpec(string) (string, error)
	Create(*specs.Spec, string) error
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
	stateFactory       StateFactory
	containerFactory   ContainerFactory
	mounter            Mounter
	hcsQuery           HCSQuery
	processWrapper     ProcessWrapper
	rootDir            string
	credentialSpecPath string
}

func New(s StateFactory, c ContainerFactory, m Mounter, h HCSQuery, p ProcessWrapper, rootDir, credentialSpecPath string) *Runtime {
	return &Runtime{
		stateFactory:       s,
		containerFactory:   c,
		mounter:            m,
		hcsQuery:           h,
		processWrapper:     p,
		rootDir:            rootDir,
		credentialSpecPath: credentialSpecPath,
	}
}

func (r *Runtime) Create(containerId, bundlePath string) error {
	logger := logrus.WithFields(logrus.Fields{
		"bundle":      bundlePath,
		"containerId": containerId,
	})
	logger.Debug("creating container")

	client := hcs.Client{}
	cm := r.containerFactory.NewManager(logger, &client, containerId)

	wsc := winsyscall.WinSyscall{}
	sm := r.stateFactory.NewManager(logger, &client, &wsc, containerId, r.rootDir)

	_, err := r.createContainer(cm, sm, bundlePath)
	return err
}

func (r *Runtime) Delete(containerId string, force bool) error {
	logger := logrus.WithFields(logrus.Fields{
		"containerId": containerId,
		"force":       force,
	})
	logger.Debug("deleting container")

	client := hcs.Client{}
	wsc := winsyscall.WinSyscall{}

	query := hcsshim.ComputeSystemQuery{Owners: []string{containerId}}
	sidecarContainerProperties, err := r.hcsQuery.GetContainers(query)
	if err != nil {
		return err
	}

	containerIdsToDelete := []string{}
	for _, sidecarContainerProperty := range sidecarContainerProperties {
		containerIdsToDelete = append(containerIdsToDelete, sidecarContainerProperty.ID)
	}
	containerIdsToDelete = append(containerIdsToDelete, containerId)

	var allErrors []string
	for _, containerIdToDelete := range containerIdsToDelete {
		cm := r.containerFactory.NewManager(logger, &client, containerIdToDelete)

		sm := r.stateFactory.NewManager(logger, &client, &wsc, containerIdToDelete, r.rootDir)

		if err := r.deleteContainer(cm, sm, force, logger); err != nil {
			allErrors = append(allErrors, err.Error())
		}
	}

	if len(allErrors) == 0 {
		return nil
	} else {
		return errors.New(strings.Join(allErrors, "\n"))
	}
}

func (r *Runtime) Events(containerId string, output io.Writer, showStats bool) error {
	logger := logrus.WithFields(logrus.Fields{
		"containerId": containerId,
	})
	logger.Debug("retrieving container events and info")

	client := hcs.Client{}
	cm := r.containerFactory.NewManager(logger, &client, containerId)

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
	logger := logrus.WithField("containerId", containerId)

	processSpec, err := config.ValidateProcess(logger, processConfigFile, processOverrides)
	if err != nil {
		return 1, err
	}

	logger = logger.WithFields(logrus.Fields{
		"processConfig": processConfigFile,
		"pidFile":       pidFile,
		"args":          processSpec.Args,
		"cwd":           processSpec.Cwd,
		"user":          processSpec.User.Username,
		"env":           processSpec.Env,
		"detach":        detach,
	})
	logger.Debug("executing process in container")

	client := hcs.Client{}
	cm := r.containerFactory.NewManager(logger, &client, containerId)

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
	logger := logrus.WithFields(logrus.Fields{
		"bundle":      bundlePath,
		"containerId": containerId,
		"pidFile":     pidFile,
		"detach":      detach,
	})
	logger.Debug("creating container")

	client := hcs.Client{}
	cm := r.containerFactory.NewManager(logger, &client, containerId)

	wsc := winsyscall.WinSyscall{}
	sm := r.stateFactory.NewManager(logger, &client, &wsc, containerId, r.rootDir)

	spec, err := r.createContainer(cm, sm, bundlePath)
	if err != nil {
		return 1, err
	}

	process, err := r.startProcess(cm, sm, spec, pidFile, detach, logger)
	if err != nil {
		return 1, err
	}
	defer process.Close()

	wrappedProcess := r.processWrapper.Wrap(process)
	if err = wrappedProcess.WritePIDFile(pidFile); err != nil {
		return 1, err
	}

	if !detach {
		s := make(chan os.Signal, 1)
		wrappedProcess.SetInterrupt(s)

		exitCode, attachErr := wrappedProcess.AttachIO(io.Stdin, io.Stdout, io.Stderr)
		deleteErr := r.deleteContainer(cm, sm, false, logger)
		if attachErr != nil {
			return exitCode, attachErr
		}

		if deleteErr != nil {
			return 1, deleteErr
		}

		return exitCode, nil
	}

	return 0, nil
}

func (r *Runtime) Start(containerId, pidFile string) error {
	logger := logrus.WithFields(logrus.Fields{
		"containerId": containerId,
		"pidFile":     pidFile,
	})
	logger.Debug("starting process in container")

	client := hcs.Client{}
	cm := r.containerFactory.NewManager(logger, &client, containerId)

	wsc := winsyscall.WinSyscall{}
	sm := r.stateFactory.NewManager(logger, &client, &wsc, containerId, r.rootDir)

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

	/*
	* When IO is attached to the process (detach=false), it is seen that
	* hcsshim will keep a handle to the process open, and therefore the
	* statemanager can do OpenProcess() to collect information about the process.
	 */
	bDetach := false
	process, err := r.startProcess(cm, sm, spec, pidFile, bDetach, logger)
	if err != nil {
		return err
	}
	defer process.Close()

	wrappedProcess := r.processWrapper.Wrap(process)
	err = wrappedProcess.WritePIDFile(pidFile)
	if err != nil {
		return err
	}

	return nil
}

func (r *Runtime) State(containerId string, output io.Writer) error {
	logger := logrus.WithFields(logrus.Fields{
		"containerId": containerId,
	})
	logger.Debug("retrieving state of container")

	client := hcs.Client{}
	wsc := winsyscall.WinSyscall{}
	sm := r.stateFactory.NewManager(logger, &client, &wsc, containerId, r.rootDir)

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

func (r *Runtime) createContainer(cm ContainerManager, sm StateManager, bundlePath string) (*specs.Spec, error) {
	spec, err := cm.Spec(bundlePath)
	if err != nil {
		return nil, err
	}

	credentialSpec, err := cm.CredentialSpec(r.credentialSpecPath)
	if err != nil {
		return nil, err
	}

	if err := cm.Create(spec, credentialSpec); err != nil {
		return nil, err
	}

	if err := sm.Initialize(bundlePath); err != nil {
		// #nosec G104 - we don't need to capture errors from deleting the thing that failed to initialize
		cm.Delete(false)
		return nil, err
	}

	return spec, nil
}

func (r *Runtime) deleteContainer(cm ContainerManager, sm StateManager, force bool, logger *logrus.Entry) error {
	var errs []string

	ociState, err := sm.State()
	if err != nil {
		logger.Error(err)

		if _, ok := err.(*hcs.NotFoundError); ok {
			if force {
				return nil
			}
			return err
		}

		errs = append(errs, err.Error())
	} else if ociState.Pid != 0 {
		if err := r.mounter.Unmount(ociState.Pid); err != nil {
			logger.Error(err)
			errs = append(errs, err.Error())
		}
	}

	if err := sm.Delete(); err != nil {
		logger.Error(err)
		errs = append(errs, err.Error())
	}

	if err := cm.Delete(force); err != nil {
		logger.Error(err)
		errs = append(errs, err.Error())
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func (r *Runtime) startProcess(cm ContainerManager, sm StateManager, spec *specs.Spec, pidFile string, detach bool, logger *logrus.Entry) (hcs.Process, error) {
	process, err := cm.Exec(spec.Process, !detach)
	if err != nil {
		if cErr, ok := errors.Cause(err).(*container.CouldNotCreateProcessError); ok {
			if sErr := sm.SetFailure(); sErr != nil {
				logger.Error(sErr)
			}
			return nil, cErr
		}
		return nil, err
	}

	if err := sm.SetSuccess(process); err != nil {
		return nil, err
	}

	if err := r.mounter.Mount(process.Pid(), spec.Root.Path, logger); err != nil {
		return nil, err
	}

	return process, nil
}
