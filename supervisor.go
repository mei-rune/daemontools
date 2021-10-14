package daemontools

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/runner-mei/cron"
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
	PROC_FAIL     = 4
)

func statusString(srv_status, proc_status int32) string {
	switch srv_status {
	case SRV_INIT:
		return "disabled"
	case SRV_STOPPING:
		return "disabling"
	case SRV_STARTING, SRV_RUNNING:
		switch proc_status {
		case PROC_INIT:
			return "waiting"
		case PROC_STARTING:
			return "starting"
		case PROC_RUNNING:
			return "running"
		case PROC_STOPPNG:
			return "crashing"
		case PROC_FAIL:
			return "crashing"
		}
	}
	return fmt.Sprintf("srv=%d, proc=%d", srv_status, proc_status)
}

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

func ToProcString(status int32) string {
	return procString(status)
}

func procString(status int32) string {
	switch status {
	case PROC_INIT:
		return "PROC_INIT"
	case PROC_STARTING:
		return "PROC_STARTING"
	case PROC_RUNNING:
		return "PROC_RUNNING"
	case PROC_FAIL:
		return "PROC_CRASHING"
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

func (self *command) command(mode string) *exec.Cmd {
	switch self.proc {
	case "__kill___", "", "__signal__", "__console__":
		return &exec.Cmd{Path: self.proc}
	default:
		cmd := exec.Command(self.proc, self.arguments...)
		cmd.Dir = self.directory
		cmd.Env = os.Environ()
		os_env := os.Environ()
		environments := make([]string, 0, len(self.environments)+len(os_env))
		if 0 != len(self.environments) {
			environments = append(environments, self.environments...)
		}

		if "" != mode {
			environments = append(environments, "DAEMON_RUN_MODE="+mode)
		}
		environments = append(environments, os_env...)
		cmd.Env = environments
		return cmd
	}
}

type Status struct {
	Name   string
	Pid    int64
	Status string
}

type supervisor interface {
	fileName() string
	name() string
	start() bool
	stop() bool
	isMode(mode string) bool
	untilStarted() error
	untilStopped() error

	setManager(mgr *Manager)
	setOutput(out io.Writer)
	stats() map[string]interface{}
	GetStatus() Status
}

type supervisorBase struct {
	cr              *cron.Cron
	file            string
	proc_name       string
	retries         int
	killTimeout     time.Duration
	start_cmd       *command
	stop_cmd        *command
	mode            string
	out             io.Writer
	srv_status      int32
	on              func(string, int32)
	restartSchedule string

	cleansBefore []string
}

func (self *supervisorBase) cleanBefore() {
	for _, cleanPath := range self.cleansBefore {
		if strings.Contains(cleanPath, "*") {
			files, err := filepath.Glob(cleanPath)
			if err != nil {
				self.logString("删除 '" + cleanPath + "' 失败: " + err.Error())
			}
			for _, name := range files {
				err := os.RemoveAll(name)
				if err != nil {
					self.logString("删除 '" + name + "' 失败: " + err.Error())
				}
			}
			return
		}
		err := os.RemoveAll(cleanPath)
		if err != nil {
			self.logString("删除 '" + cleanPath + "' 失败: " + err.Error())
		}
	}
}

func (self *supervisorBase) setManager(mgr *Manager) {
	self.cr = mgr.cr
}

func (self *supervisorBase) onEvent(status int32) {
	if self.on != nil {
		self.on(self.name(), status)
	}
}

func (self *supervisorBase) fileName() string {
	return self.file
}

func (self *supervisorBase) isMode(mode string) bool {
	if "" == mode || "all" == mode {
		return true
	}
	if "" == self.mode || "default" == self.mode {
		return true
	}
	if self.mode == mode {
		return true
	}
	return false
}

func (self *supervisorBase) name() string {
	return self.proc_name
}

func (self *supervisorBase) stats() map[string]interface{} {
	status := atomic.LoadInt32(&self.srv_status)
	return map[string]interface{}{
		"name":         self.proc_name,
		"retries":      self.retries,
		"kill_timeout": self.killTimeout,
		"owned":        true,
		"is_started":   (status != SRV_INIT) && (status != SRV_STOPPING),
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

	var sig os.Signal
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
			e = execWithTimeout(self.killTimeout, self.stop_cmd.command(self.mode))
			if nil != e {
				return false, e.Error()
			}
			return true, ""
		}
		return false, e.Error()
	}
	defer pr.Release()

	started := time.Now()
	e = execWithTimeout(self.killTimeout, self.stop_cmd.command(self.mode))
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
