package daemontools

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

type supervisorWithPidfile struct {
	supervisorBase

	owner      int32
	last_error error
	pidfile    string
}

func (self *supervisorWithPidfile) stats() map[string]interface{} {
	srv_status := self.status()
	res := self.supervisorBase.stats()
	res["pidfile"] = self.pidfile
	res["status"] = srv_status

	res["owned"] = (1 == atomic.LoadInt32(&self.owner))
	res["srv_status"] = srv_status
	return res
}

func (self *supervisorWithPidfile) status() string {
	s := atomic.LoadInt32(&self.srv_status)
	if SRV_RUNNING == s {
		if 1 != atomic.LoadInt32(&self.owner) {
			return "running, extern"
		}
	}
	return srvString(s)
}

func (self *supervisorWithPidfile) start() bool {
	if self.pidFileExists() {
		self.logString("[sys] ==================== srv  start ====================\r\n")
		self.logString("[sys] process is already started by other\r\n")
		atomic.CompareAndSwapInt32(&self.srv_status, SRV_INIT, SRV_RUNNING)
		atomic.StoreInt32(&self.owner, 0)
		return false
	}

	if !atomic.CompareAndSwapInt32(&self.srv_status, SRV_INIT, SRV_STARTING) {
		return false
	}

	defer func() {
		if o := recover(); nil != o {
			atomic.CompareAndSwapInt32(&self.srv_status, SRV_STARTING, PROC_INIT)
			self.last_error = errors.New(fmt.Sprint(o))
		} else {
			self.last_error = nil
		}
	}()

	self.logString("[sys] ==================== srv  start ====================\r\n")
	for i := 0; i < self.retries; i++ {
		self.run(func() {
			if atomic.CompareAndSwapInt32(&self.srv_status, SRV_STARTING, SRV_RUNNING) {
				atomic.StoreInt32(&self.owner, 1)
			}
		})

		if SRV_STARTING != atomic.LoadInt32(&self.srv_status) {
			return false
		}
	}

	atomic.CompareAndSwapInt32(&self.srv_status, SRV_STARTING, PROC_INIT)

	return true
}

func (self *supervisorWithPidfile) stop() bool {
	if 1 != atomic.LoadInt32(&self.owner) {
		atomic.CompareAndSwapInt32(&self.srv_status, SRV_RUNNING, SRV_INIT)
		self.logString("[sys] ignore process\r\n")
		self.logString("[sys] ====================  srv  end  ====================\r\n")
		return false
	}

	defer func() {
		if o := recover(); nil != o {
			self.last_error = errors.New(fmt.Sprint(o))
		} else {
			self.last_error = nil
		}
	}()

	if !atomic.CompareAndSwapInt32(&self.srv_status, SRV_RUNNING, SRV_STOPPING) &&
		!atomic.CompareAndSwapInt32(&self.srv_status, SRV_STARTING, SRV_STOPPING) {
		return false
	}

	self.interrupt()
	atomic.CompareAndSwapInt32(&self.srv_status, SRV_STOPPING, PROC_INIT)
	atomic.StoreInt32(&self.owner, 0)
	self.logString("[sys] ====================  srv  end  ====================\r\n")

	return true
}

func (self *supervisorWithPidfile) untilStarted() error {
	return self.untilWith(SRV_RUNNING)
}

func (self *supervisorWithPidfile) untilStopped() error {
	return self.untilWith(SRV_INIT)
}

func (self *supervisorWithPidfile) untilWith(excepted int32) error {
	if nil != self.last_error {
		return self.last_error
	}
	s := atomic.LoadInt32(&self.srv_status)
	if excepted == s {
		return nil
	}

	return fmt.Errorf("status is invalid, excepted is %v, actual is %v.",
		srvString(excepted), srvString(s))
}

func (self *supervisorWithPidfile) interrupt() {
	var ok bool
	var txt string

	if nil == self.stop_cmd {
		return
	}

	pid, _ := self.readPidfile()
	switch self.stop_cmd.proc {
	case "__signal__":
		ok, txt = self.killBySignal(pid)
	default:
		ok, txt = self.killByCmd(pid)
	}

	if ok {
		if nil != self.out && 0 != len(txt) {
			io.WriteString(self.out, txt)
		}
		return
	} else {
		self.logString(txt + "\r\n")
	}
}

func (self *supervisorWithPidfile) run(cb func()) {
	cmd := self.start_cmd.command(self.mode)
	//if 0 == len(self.prompt) {
	if *is_print {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = self.out
		cmd.Stderr = self.out
	}
	// } else {
	// 	wrapped := wrap(self.out, []byte(self.prompt), cb)
	// 	cmd.Stdout = wrapped
	// 	cmd.Stderr = wrapped
	// }

	if e := cmd.Start(); nil != e {
		self.logString(fmt.Sprintf("[sys] start process failed - %v\r\n", e))
		return
	}

	if _, e := cmd.Process.Wait(); nil != e {
		self.logString(fmt.Sprintf("[sys] wait process failed - %v\r\n", e))
		return
	}

	go func() {
		cmd.Wait()
		// if e = cmd.Wait(); nil != e {
		// 	self.logString(fmt.Sprintf("[sys] wait process failed - %v\r\n", e))
		// 	return
		// }
	}()

	for i := 0; i < 10; i++ {
		if self.pidFileExists() {
			if nil != cb {
				cb()
			}
			return
		}
		time.Sleep(1 * time.Second)
	}

	self.logString(fmt.Sprintf("[sys] pid file '%v' is not exists.\r\n", self.pidfile))
}

func (self *supervisorWithPidfile) pidFileExists() bool {
	_, err := os.Stat(self.pidfile)
	if nil == err {
		return true
	}
	if os.IsPermission(err) {
		return true
	}
	return false
}

func (self *supervisorWithPidfile) readPidfile() (int, error) {
	fd, e := os.Open(self.pidfile)
	if nil != e {
		_, err := os.Stat(self.pidfile)
		if nil == err {
			return 0, errors.New("open pid file '" + self.pidfile + "' is error, permission error, " + e.Error())
		} else {
			return 0, errors.New("pid file '" + self.pidfile + "' is not exists? " + e.Error())
		}
	}

	reader := textproto.NewReader(bufio.NewReader(fd))
	line, e := reader.ReadLine()
	if nil != e {
		return 0, errors.New("read pid file '" + self.pidfile + "' failed, " + e.Error())
	}

	pid, e := strconv.ParseInt(line, 10, 0)
	if nil != e {
		return 0, errors.New("read pid file '" + self.pidfile + "' failed, " + e.Error())
	}

	return int(pid), nil
}
