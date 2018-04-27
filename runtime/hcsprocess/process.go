package hcsprocess

import (
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/Microsoft/hcsshim"
)

type Process struct {
	process hcsshim.Process
}

func New(p hcsshim.Process) *Process {
	return &Process{process: p}
}

func (p *Process) WritePIDFile(pidFile string) error {
	if pidFile != "" {
		if err := ioutil.WriteFile(pidFile, []byte(strconv.FormatInt(int64(p.process.Pid()), 10)), 0666); err != nil {
			return err
		}
	}
	return nil
}

func (p *Process) AttachIO(attachStdin io.Reader, attachStdout, attachStderr io.Writer) (int, error) {
	stdin, stdout, stderr, err := p.process.Stdio()
	if err != nil {
		return -1, err
	}

	var wg sync.WaitGroup

	if attachStdin != nil {
		wg.Add(1)
		go func() {
			_, _ = io.Copy(stdin, attachStdin)
			_ = stdin.Close()
			p.process.CloseStdin()
			wg.Done()
		}()
	} else {
		_ = stdin.Close()
		p.process.CloseStdin()
	}

	if attachStdout != nil {
		wg.Add(1)
		go func() {
			_, _ = io.Copy(attachStdout, stdout)
			_ = stdout.Close()
			wg.Done()
		}()
	} else {
		_ = stdout.Close()
	}

	if attachStderr != nil {
		wg.Add(1)
		go func() {
			_, _ = io.Copy(attachStderr, stderr)
			_ = stderr.Close()
			wg.Done()
		}()
	} else {
		_ = stderr.Close()
	}

	err = p.process.Wait()
	waitWithTimeout(&wg, 1*time.Second)
	if err != nil {
		return -1, err
	}

	return p.process.ExitCode()
}

func (p *Process) SetInterrupt(s chan os.Signal) {
	signal.Notify(s, os.Interrupt)
	go func() {
		<-s
		p.process.Kill()
	}()
}

func waitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) {
	wgEmpty := make(chan interface{}, 1)
	go func() {
		wg.Wait()
		wgEmpty <- nil
	}()

	select {
	case <-time.After(timeout):
	case <-wgEmpty:
	}
}
