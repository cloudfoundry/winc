package process

import (
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Microsoft/hcsshim"
)

type Client struct {
	process hcsshim.Process
}

func NewClient(p hcsshim.Process) *Client {
	return &Client{process: p}
}

func (m *Client) WritePIDFile(pidFile string) error {
	if pidFile != "" {
		if err := ioutil.WriteFile(pidFile, []byte(strconv.FormatInt(int64(m.process.Pid()), 10)), 0666); err != nil {
			return err
		}
	}
	return nil
}

func (m *Client) AttachIO(attachStdin io.Reader, attachStdout, attachStderr io.Writer) (int, error) {
	stdin, stdout, stderr, err := m.process.Stdio()
	if err != nil {
		return -1, err
	}

	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		_, _ = io.Copy(stdin, attachStdin)
		_ = stdin.Close()
		wg.Done()
	}()
	go func() {
		wg.Add(1)
		_, _ = io.Copy(attachStdout, stdout)
		_ = stdout.Close()
		wg.Done()
	}()
	go func() {
		wg.Add(1)
		_, _ = io.Copy(attachStderr, stderr)
		_ = stderr.Close()
		wg.Done()
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		_ = m.process.Kill()
	}()

	err = m.process.Wait()
	waitWithTimeout(&wg, 1*time.Second)
	if err != nil {
		return -1, err
	}

	// !! DELETE WAS HERE

	return m.process.ExitCode()
}

func (m *Client) StartTime(pid uint32) (syscall.Filetime, error) {
	//pid := uint32(m.process.Pid())
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, pid)
	if err != nil {
		return syscall.Filetime{}, err
	}
	defer syscall.CloseHandle(h)

	var (
		creationTime syscall.Filetime
		exitTime     syscall.Filetime
		kernelTime   syscall.Filetime
		userTime     syscall.Filetime
	)

	if err := syscall.GetProcessTimes(h, &creationTime, &exitTime, &kernelTime, &userTime); err != nil {
		return syscall.Filetime{}, err
	}

	return creationTime, nil
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
