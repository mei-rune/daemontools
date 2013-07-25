package daemontools

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/textproto"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
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

type supervisor struct {
	name        string
	prompt      string
	pidfile     string
	repected    int
	killTimeout time.Duration
	start       *command
	stop        *command

	out         io.Writer
	srv_status  int32
	proc_status int32
	pid         int
	stdin       io.Writer

	lock sync.Mutex
	cond *sync.Cond
	once sync.Once
}

func (self *supervisor) stats() interface{} {
	return map[string]interface{}{
		"name":         self.name,
		"prompt":       self.prompt,
		"repected":     self.repected,
		"kill_timeout": self.killTimeout,
		"srv_status":   srvString(atomic.LoadInt32(&self.srv_status)),
		"proc_status":  procString(atomic.LoadInt32(&self.proc_status))}
}

func (self *supervisor) init() {
	self.once.Do(func() {
		self.cond = sync.NewCond(&self.lock)
	})
}
func (self *supervisor) Start() {
	self.init()

	if !self.casStatus(SRV_INIT, SRV_STARTING) {
		return
	}

	go self.loop()
}

func (self *supervisor) casStatus(old_status, new_status int32) bool {
	if !atomic.CompareAndSwapInt32(&self.srv_status, old_status, new_status) {
		return false
	}

	self.cond.Broadcast()
	return true
}

func (self *supervisor) setStatus(new_status int32) {
	atomic.StoreInt32(&self.srv_status, new_status)
	self.cond.Broadcast()
}

func (self *supervisor) UntilStarted() error {
	return self.untilWith(SRV_STARTING, SRV_RUNNING)
}

func (self *supervisor) UntilStopped() error {
	return self.untilWith(SRV_STOPPING, SRV_INIT)
}

func (self *supervisor) untilWith(old_status, srv_status int32) error {
	self.init()

	self.cond.L.Lock()
	defer self.cond.L.Unlock()

	for {
		s := atomic.LoadInt32(&self.srv_status)
		switch s {
		case srv_status:
			return nil
		case old_status:
			break
		default:
			return fmt.Errorf("status is invalid, old_status is %v, excepted is %v, actual is %v.",
				srvString(old_status), srvString(srv_status), srvString(s))
		}
		self.cond.Wait()
	}
}

func (self *supervisor) Stop() {
	self.init()

	if !self.casStatus(SRV_RUNNING, SRV_STOPPING) &&
		!self.casStatus(SRV_STARTING, SRV_STOPPING) {
		return
	}

	self.interrupt()
}

func (self *supervisor) interrupt() {
	pid := 0
	self.cond.L.Lock()
	pid = self.pid
	self.cond.L.Unlock()

	if 0 == pid {
		return
	}
	var ok bool
	var txt string

	if nil != self.stop {
		switch self.stop.proc {
		case "__kill___", "":
			goto end
		case "__signal__":
			ok, txt = self.killBySignal(pid)
		case "__console__":
			ok, txt = self.killByConsole(pid)
		default:
			ok, txt = self.killByCmd(pid)
		}

		if ok {
			if 0 != len(txt) {
				self.out.Write([]byte(txt))
			}
			return
		}
	}
end:
	e := kill(pid)
	if 0 != len(txt) {
		txt = txt + "\r\n"
	}
	if nil != e {
		txt = txt + "[sys]" + e.Error() + "\r\n"
	} else {
		txt = txt + "[sys] kill process when exit\r\n"
	}

	if 0 != len(txt) {
		self.logString(txt)
	}
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

func (self *supervisor) killBySignal(pid int) (bool, string) {
	if nil == self.stop.arguments || 0 == len(self.stop.arguments) {
		return false, "signal is empty"
	}

	var sig os.Signal = nil
	switch self.stop.arguments[0] {
	case "kill":
		sig = os.Kill
		break
	case "int":
		sig = os.Interrupt
		break
	default:
		return false, "signal '" + self.stop.arguments[0] + "' is unsupported"
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

func (self *supervisor) killByConsole(pid int) (bool, string) {

	if nil == self.stop.arguments || 0 == len(self.stop.arguments) {
		return false, "console arguments is empty"
	}

	e := func() error {
		self.cond.L.Lock()
		defer self.cond.L.Unlock()
		if nil == self.stdin {
			return errors.New("stdin is not redirect.")
		}

		for _, s := range self.stop.arguments {
			_, e := self.stdin.Write([]byte(s + "\r\n"))
			if nil != e {
				return e
			}
		}
		return nil
	}()

	if nil != e {
		return false, e.Error()
	}

	pr, e := os.FindProcess(pid)
	if nil != e {
		return false, e.Error()
	}
	e = waitWithTimeout(self.killTimeout, pr)
	if nil != e {
		return false, e.Error()
	}
	return true, ""
}

func (self *supervisor) killByCmd(pid int) (bool, string) {
	pr, e := os.FindProcess(pid)
	if nil != e {
		return false, e.Error()
	}

	started := time.Now()
	e = execWithTimeout(self.killTimeout, self.stop.command())
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

func (self *supervisor) logString(msg string) {
	if *is_print {
		fmt.Print(msg)
		return
	}
	if nil == self.out {
		return
	}
	_, err := io.WriteString(self.out, msg)
	if nil != err {
		log.Printf("[sys] write exception to file error - %v\r\n", err)
	}
}

func (self *supervisor) loop() {
	defer func() {
		self.cond.L.Lock()
		self.stdin = nil
		self.pid = 0
		self.cond.L.Unlock()

		self.setStatus(SRV_INIT)
		atomic.StoreInt32(&self.proc_status, PROC_INIT)

		if e := recover(); nil != e {
			var buffer bytes.Buffer
			buffer.WriteString(fmt.Sprintf("[panic] crashed with error - %s\r\n", e))
			for i := 1; ; i += 1 {
				_, file, line, ok := runtime.Caller(i)
				if !ok {
					break
				}
				buffer.WriteString(fmt.Sprintf("    %s:%d\r\n", file, line))
			}
			self.logString(buffer.String())
		}

		self.logString("[sys] ====================  srv  end  ====================\r\n")
	}()

	self.logString("[sys] ==================== srv  start ====================\r\n")
	for i := 0; i < self.repected; i++ {
		self.run(func() {
			self.casStatus(SRV_STARTING, SRV_RUNNING)
		})
		if SRV_STARTING != atomic.LoadInt32(&self.srv_status) {
			break
		}
	}

	for SRV_RUNNING == atomic.LoadInt32(&self.srv_status) {
		time.Sleep(time.Second)
		self.run(nil)
	}
}

func (self *supervisor) utilExitedBySleep(pid int) bool {
	for SRV_RUNNING == atomic.LoadInt32(&self.srv_status) {
		time.Sleep(time.Second)
		self.run(nil)
		pr, e := os.FindProcess(int(pid))
		if nil != e {
			if os.IsPermission(e) {
			}
		}
	}
}

func (self *supervisor) utilExitedByWait(pid int) bool {
	pr, e := os.FindProcess(int(pid))
	if nil != e {
		if os.IsPermission(e) {
			if nil != cb {
				cb()
			}

			self.cond.L.Lock()
			if is_owner {
				self.pid = int(pid)
			} else {
				self.pid = 0
			}
			self.cond.L.Unlock()

			self.waitProcess(pid)
		} else {
			self.logString("[sys] find process with pid was '" + line + "' failed, " + e.Error() + "\r\n")
		}
		return false
	}
	defer pr.Release()

	if nil != cb {
		cb()
	}

	self.cond.L.Lock()
	if is_owner {
		self.pid = int(pid)
	} else {
		self.pid = 0
	}
	self.cond.L.Unlock()

	_, e = pr.Wait()
	if nil != e {
		self.logString("[sys] find process with pid was '" + line + "' failed, " + e.Error() + "\r\n")
	}
	self.logString("[sys] process '" + line + "' is exited.\r\n")
	return false
}

func (self *supervisor) runPidfile(cb func(), is_owner bool) bool {
	if 0 == len(self.pidfile) {
		return true
	}

	fd, e := os.Open(self.pidfile)
	if nil != e {
		_, err := os.Stat(self.pidfile)
		if nil == err {
			self.logString("[sys] open pid file '" + self.pidfile + "' is error, permission error, " + e.Error() + "\r\n")
		} else {
			self.logString("[sys] pid file '" + self.pidfile + "' is not exists? " + e.Error() + "\r\n")
		}
		return true
	}

	reader := textproto.NewReader(bufio.NewReader(fd))
	line, e := reader.ReadLine()
	if nil != e {
		self.logString("[sys] read pid file '" + self.pidfile + "' failed, " + e.Error() + "\r\n")
		return false
	}

	pid, e := strconv.ParseInt(line, 10, 0)
	if nil != e {
		self.logString("[sys] read pid file '" + self.pidfile + "' failed, " + e.Error() + "\r\n")
		return false
	}

	self.logString("[sys] read pid file '" + self.pidfile + "' ok, pid = " + line + "\r\n")

	if !is_owner {
		self.cond.L.Lock()
		self.pid = 0
		self.stdin = nil
		self.cond.L.Unlock()

		return self.utilExitedBySleep(pid)
	} else {
		self.cond.L.Lock()
		self.pid = int(pid)
		self.stdin = nil
		self.cond.L.Unlock()
		return self.utilExitedByWait(pid)
	}
}

func (self *supervisor) run(cb func()) {
	defer func() {

		self.cond.L.Lock()
		self.stdin = nil
		self.pid = 0
		self.cond.L.Unlock()

		atomic.StoreInt32(&self.proc_status, PROC_INIT)

		if e := recover(); nil != e {
			var buffer bytes.Buffer
			buffer.WriteString(fmt.Sprintf("[panic] crashed with error - %s\r\n", e))
			for i := 1; ; i += 1 {
				_, file, line, ok := runtime.Caller(i)
				if !ok {
					break
				}
				buffer.WriteString(fmt.Sprintf("    %s:%d\r\n", file, line))
			}

			self.logString(buffer.String())
		}
		self.logString("[sys] --------------------  proc end  --------------------\r\n")
	}()

	self.logString("[sys] -------------------- proc start --------------------\r\n")

	if !self.runPidfile(cb, false) {
		return
	}

	cmd := self.start.command()
	if 0 == len(self.prompt) {
		if *is_print {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		} else {
			cmd.Stdout = self.out
			cmd.Stderr = self.out
		}
		if nil != cb {
			cb()
		}
	} else {
		wrapped := wrap(self.out, []byte(self.prompt), cb)
		cmd.Stdout = wrapped
		cmd.Stderr = wrapped
	}

	var in io.Writer = nil
	var e error = nil
	if nil != self.stop && "__console__" == self.stop.proc {
		in, e = cmd.StdinPipe()
		if nil != e {
			self.logString(fmt.Sprintf("[sys] create pipe failed for stdin - %v\r\n", e))
		}
	}

	if e = cmd.Start(); nil != e {
		self.logString(fmt.Sprintf("[sys] start process failed - %v\r\n", e))
		return
	}

	self.cond.L.Lock()
	self.stdin = in
	self.pid = cmd.Process.Pid
	self.cond.L.Unlock()
	if e = cmd.Wait(); nil != e {
		self.logString(fmt.Sprintf("[sys] wait process failed - %v\r\n", e))
		return
	}

	self.cond.L.Lock()
	self.stdin = nil
	self.cond.L.Unlock()
	self.runPidfile(cb, true)
}
