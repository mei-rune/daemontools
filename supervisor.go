package daemontools

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync/atomic"
	"time"
)

const (
	SRV_INIT     = 0
	SRV_STARTING = 1
	SRV_RUNNING  = 2
	SRV_STOPPING = 3

	PROC_INIT     = 0
	PROC_STARTING = 1
	PROC_RUNNING  = 2
	PROC_STOPPNG  = 3
)

func srvString(status int32) string {
	switch status {
	case SRV_INIT:
		return "SRV_INIT"
	case SRV_STARTING:
		return "SRV_STARTING"
	case SRV_RUNNING:
		return "SRV_RUNNING"
	case SRV_STOPPING:
		return "SRV_STOPPING"
	}
	return fmt.Sprintf("%d", status)
}

func procString(status int32) string {
	switch status {
	case PROC_INIT:
		return "PROC_INIT"
	case PROC_STARTING:
		return "PROC_STARTING"
	case PROC_RUNNING:
		return "PROC_RUNNING"
	case PROC_STOPPNG:
		return "PROC_STOPPING"
	}
	return fmt.Sprintf("%d", status)
}

type command struct {
	proc         string
	arguments    []string
	environments []string
	directory    string
}

func (self *command) command() *exec.Cmd {
    switch self.proc {
	case "__kill___", "", "__signal__", "__console__":
	    return &exec.Cmd{Path:self.proc}
	default:
    	cmd := exec.Command(self.proc, self.arguments...)
    	cmd.Dir = self.directory
    	cmd.Env = os.Environ()
    	if nil != self.environments && 0 != len(self.environments) {
    		os_env := os.Environ()
    		environments := make([]string, 0, len(self.environments)+len(os_env))
    		environments = append(environments, self.environments...)
    		environments = append(environments, os_env...)
    		cmd.Env = environments
    	} else {
    		cmd.Env = os.Environ()
    	}
    	return cmd
	}
}

type supervisor interface {
	name() string
	start()
	stop()
	untilStarted() error
	untilStopped() error

	setOutput(out io.Writer)
	stats() map[string]interface{}
}

type supervisorBase struct {
	proc_name   string
	retries     int
	killTimeout time.Duration
	start_cmd   *command
	stop_cmd    *command

	out        io.Writer
	srv_status int32
}

func (self *supervisorBase) name() string {
	return self.proc_name
}

func (self *supervisorBase) stats() map[string]interface{} {
	status := atomic.LoadInt32(&self.srv_status)
	is_started := status != SRV_INIT || status != SRV_STOPPING
	return map[string]interface{}{
		"name":         self.proc_name,
		"retries":      self.retries,
		"kill_timeout": self.killTimeout,
		"owned":        true,
		"is_started":   is_started,
		"srv_status":   srvString(status)}
}

func (self *supervisorBase) setOutput(out io.Writer) {
	self.out = out
}

func waitWithTimeout(timeout time.Duration, pr *os.Process) error {
	errc := make(chan error, 1)
	go func() {
		_, e := pr.Wait()
		errc <- e
	}()

	var err error
	select {
	case <-time.After(timeout):
		err = fmt.Errorf("timed out after %v", timeout)
	case err = <-errc:
	}
	return err
}

func (self *supervisorBase) killBySignal(pid int) (bool, string) {
	if nil == self.stop_cmd.arguments || 0 == len(self.stop_cmd.arguments) {
		return false, "signal is empty"
	}

	var sig os.Signal = nil
	switch self.stop_cmd.arguments[0] {
	case "kill":
		sig = os.Kill
		break
	case "int":
		sig = os.Interrupt
		break
	default:
		return false, "signal '" + self.stop_cmd.arguments[0] + "' is unsupported"
	}

	pr, e := os.FindProcess(pid)
	if nil != e {
		return false, e.Error()
	}
	e = pr.Signal(sig)
	if nil != e {
		return false, e.Error()
	}
	e = waitWithTimeout(self.killTimeout, pr)
	if nil != e {
		return false, e.Error()
	}
	return true, ""
}

func (self *supervisorBase) killByCmd(pid int) (bool, string) {
	pr, e := os.FindProcess(pid)
	if nil != e {
		if os.IsPermission(e) {
			e = execWithTimeout(self.killTimeout, self.stop_cmd.command())
			if nil != e {
				return false, e.Error()
			}
			return true, ""
		}
		return false, e.Error()
	}
	defer pr.Release()

	started := time.Now()
	e = execWithTimeout(self.killTimeout, self.stop_cmd.command())
	if nil != e {
		return false, e.Error()
	}

	used := time.Now().Sub(started)
	if used >= self.killTimeout {
		return false, fmt.Sprintf("timed out after %v", self.killTimeout)
	}

	e = waitWithTimeout(self.killTimeout-used, pr)
	if nil != e {
		return false, e.Error()
	}
	return true, ""
}

func (self *supervisorBase) logString(msg string) {
	if *is_print {
		fmt.Print(msg)
		return
	}
	if nil == self.out {
		return
	}
	_, err := io.WriteString(self.out, "["+self.proc_name+"]"+msg)
	if nil != err {
		log.Printf("[sys] write exception to file error - %v\r\n", err)
	}
}
