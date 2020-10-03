package daemontools

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type supervisor_default struct {
	supervisorBase
	success_flag string

	proc_status int32
	pid         int
	stdin       io.WriteCloser

	lock sync.Mutex
	cond *sync.Cond
	once sync.Once
}

func (self *supervisor_default) closeStdin() {
	self.cond.L.Lock()
	defer self.cond.L.Unlock()
	if nil == self.stdin {
		return
	}
	self.stdin.Close()
	self.stdin = nil
}

func (self *supervisor_default) stats() map[string]interface{} {
	self.init()
	pid := 0
	self.cond.L.Lock()
	pid = self.pid
	self.cond.L.Unlock()

	srv_status := atomic.LoadInt32(&self.srv_status)
	proc_status := atomic.LoadInt32(&self.proc_status)
	res := self.supervisorBase.stats()

	res["pid"] = pid
	res["success_flag"] = self.success_flag
	res["status"] = statusString(srv_status, proc_status)
	res["srv_status"] = srvString(srv_status)
	res["proc_status"] = procString(proc_status)
	return res
}

func (self *supervisor_default) casStatus(old_status, new_status int32) bool {
	if !atomic.CompareAndSwapInt32(&self.srv_status, old_status, new_status) {
		return false
	}

	self.cond.Broadcast()
	return true
}

func (self *supervisor_default) setStatus(new_status int32) {
	atomic.StoreInt32(&self.srv_status, new_status)
	self.cond.Broadcast()
}

func (self *supervisor_default) untilStarted() error {
	return self.untilWith(SRV_STARTING, SRV_RUNNING)
}

func (self *supervisor_default) untilStopped() error {
	return self.untilWith(SRV_STOPPING, SRV_INIT)
}

func (self *supervisor_default) untilWith(old_status, srv_status int32) error {
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
			return fmt.Errorf("status is invalid, old_status is %v, excepted is %v, actual is %v",
				srvString(old_status), srvString(srv_status), srvString(s))
		}
		self.cond.Wait()
	}
}

func (self *supervisor_default) init() {
	self.once.Do(func() {
		self.cond = sync.NewCond(&self.lock)
	})
}

func (self *supervisor_default) start() {
	self.init()

	if !self.casStatus(SRV_INIT, SRV_STARTING) {
		if err := self.untilStopped(); err != nil {
			log.Println("[system]", err)
			return
		}
		if !self.casStatus(SRV_INIT, SRV_STARTING) {
			return
		}
	}

	go self.loop()
}

func (self *supervisor_default) stop() {
	self.init()
	self.logString(time.Now().String() + " [sys]swithing to '" + srvString(atomic.LoadInt32(&self.srv_status)) + "'\r\n")
	if !self.casStatus(SRV_RUNNING, SRV_STOPPING) &&
		!self.casStatus(SRV_STARTING, SRV_STOPPING) {
		return
	}
	self.logString(time.Now().String() + " [sys]swith to '" + srvString(atomic.LoadInt32(&self.srv_status)) + "'\r\n")
	go self.interrupt()
}

func (self *supervisor_default) interrupt() {
	pid := 0
	self.cond.L.Lock()
	pid = self.pid
	self.cond.L.Unlock()

	if 0 == pid {
		self.logString(time.Now().String() + " [sys] pid = 0\r\n")
		return
	}
	var ok bool
	var txt string

	if nil != self.stop_cmd {
		switch self.stop_cmd.proc {
		case "__kill___", "":
			goto end
		case "__signal__":
			ok, txt = self.killBySignal(pid)
		case "__console__":
			ok, txt = self.killByConsole(pid)
		default:
			ok, txt = self.killByCmd(pid)
		}

		if 0 != len(txt) {
			self.logString(txt)
		}
		if ok {
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

func (self *supervisor_default) killByConsole(pid int) (bool, string) {

	if nil == self.stop_cmd.arguments || 0 == len(self.stop_cmd.arguments) {
		return false, "console arguments is empty"
	}

	e := func() error {
		self.cond.L.Lock()
		defer self.cond.L.Unlock()
		if nil == self.stdin {
			return errors.New("stdin is not redirect")
		}

		defer func() {
			self.stdin.Close()
			self.stdin = nil
		}()
		for _, s := range self.stop_cmd.arguments {
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

func (self *supervisor_default) loop() {
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
			for i := 1; ; i++ {
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

	var jobID = self.name() + "-" + time.Now().Format(time.RFC3339Nano)
	if self.restartSchedule != "" {
		if err := self.cr.AddFunc(jobID, self.restartSchedule, self.interrupt); err != nil {
			self.logString("[sys] " + err.Error() + "\r\n")
		} else {
			defer self.cr.Unschedule(jobID)
			self.logString("[sys] " + jobID + "\r\n")
		}
	}

	self.logString("[sys] ==================== srv  start ====================\r\n")

	retries := self.retries
	if retries <= 0 {
		retries = 1
	}
	for i := 0; i < retries; i++ {
		self.run(func() {
			self.casStatus(SRV_STARTING, SRV_RUNNING)
		})
		if SRV_STARTING != atomic.LoadInt32(&self.srv_status) {
			break
		}

		self.logString(time.Now().String() + " [sys]current status is '" + srvString(atomic.LoadInt32(&self.srv_status)) + "'\r\n")
	}

	for SRV_RUNNING == atomic.LoadInt32(&self.srv_status) {
		self.logString(time.Now().String() + " [sys]current status is '" + srvString(atomic.LoadInt32(&self.srv_status)) + "'\r\n")
		time.Sleep(2 * time.Second)
		self.run(nil)
	}
}

func (self *supervisor_default) run(cb func()) {
	self.cond.L.Lock()
	isLocked := true
	defer func() {
		if !isLocked {
			self.cond.L.Lock()
		}
		self.stdin = nil
		self.pid = 0
		self.cond.L.Unlock()

		self.onEvent(PROC_STOPPNG)
		atomic.StoreInt32(&self.proc_status, PROC_INIT)

		if e := recover(); nil != e {
			var buffer bytes.Buffer
			buffer.WriteString(fmt.Sprintf("[panic] crashed with error - %s\r\n", e))
			for i := 1; ; i++ {
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

	if st := atomic.LoadInt32(&self.srv_status); SRV_RUNNING != st && SRV_STARTING != st {
		return
	}

	self.logString("[sys] -------------------- proc start --------------------\r\n")
	atomic.StoreInt32(&self.proc_status, PROC_STARTING)

	self.cleanBefore()

	cmd := self.start_cmd.command(self.mode)
	if self.success_flag == "" {
		if *is_print {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		} else {
			cmd.Stdout = self.out
			cmd.Stderr = self.out
		}
	} else {
		wrapped := wrap(self.out, []byte(self.success_flag), cb)
		cmd.Stdout = wrapped
		cmd.Stderr = wrapped
	}

	var in io.WriteCloser
	var e error
	if nil != self.stop_cmd && "__console__" == self.stop_cmd.proc {
		in, e = cmd.StdinPipe()
		if nil != e {
			self.logString(fmt.Sprintf("[sys] create pipe failed for stdin - %v\r\n", e))
		}
	}

	self.logString(fmt.Sprintf("[sys] %v\r\n", cmd.Path))
	for idx, s := range cmd.Args {
		if 0 == idx {
			continue
		}
		self.logString(fmt.Sprintf("[sys] \t\t%v\r\n", s))
	}

	self.onEvent(PROC_STARTING)
	if e = cmd.Start(); nil != e {

		if self.success_flag == "" {
			if nil != cb {
				cb()
			}
		}

		self.logString(fmt.Sprintf("[sys] start process failed - %v\r\n", e))
		return
	}
	atomic.StoreInt32(&self.proc_status, PROC_RUNNING)
	self.stdin = in
	self.pid = cmd.Process.Pid
	self.cond.L.Unlock()
	isLocked = false
	if self.success_flag == "" {
		if nil != cb {
			cb()
		}
	}

	self.onEvent(PROC_RUNNING)

	// cmd.Wait() may blocked for ever in the win32.
	ch := make(chan error, 3)
	go func() {
		ch <- cmd.Wait()
	}()

	tricker := time.NewTicker(1 * time.Minute)
	defer tricker.Stop()
	isStopped := false
	for !isStopped {
		select {
		case e = <-ch:
			isStopped = true
		case <-tricker.C:
			if !IsInProcessList(self.pid, "") {
				isStopped = true
				self.closeStdin()
				self.logString("[sys] process pid('" + strconv.FormatInt(int64(self.pid), 10) + "') is not found.\r\n")
			}
		}
	}
	if nil != e {
		self.logString(fmt.Sprintf("[sys] wait process failed - %v\r\n", e))
		return
	}

	self.logString("[sys] process is exited.\r\n")
}
