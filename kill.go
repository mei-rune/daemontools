package daemontools

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/go-ps"
)

func killByPid(pid int) error {
	pr, e := os.FindProcess(pid)
	if nil != e {
		return e
	}
	defer pr.Release()
	return pr.Kill()
}
func killProcessAndChildren(pid int, processes []ps.Process) error {
	if -1 == pid {
		return nil
	}
	if nil == processes {
		var e error
		processes, e = ps.Processes()
		if nil != e {
			log.Println("killProcessAndChildren()" + e.Error())
			return killByPid(pid)
		}
	}

	for _, pr := range processes {
		if pr.PPid() == pid {
			killProcessAndChildren(pr.Pid(), processes)
		}
	}
	return killByPid(pid)
}

func kill(pid int) error {
	return killProcessAndChildren(pid, nil)
}

func execWithTimeout(timeout time.Duration, cmd *exec.Cmd) error {
	var err error
	if err = cmd.Start(); nil != err {
		return err
	}

	errc := make(chan error, 1)
	go func() {
		errc <- cmd.Wait()
		close(errc)
	}()

	select {
	case <-time.After(timeout):
		cmd.Process.Kill()
		err = fmt.Errorf("timed out after %v", timeout)
	case err = <-errc:
	}
	return err
}

func processExists(pid int, image string) bool {
	pr, e := ps.FindProcess(pid)
	if nil != e {
		panic(e)
	}

	if nil == pr {
		return false
	}

	if "" != image && !strings.Contains(strings.ToLower(pr.Executable()), strings.ToLower(image)) {
		return false
	}
	return true
}

var procs_lock sync.Mutex
var procs_list []ps.Process

func init() {
	time.AfterFunc(1*time.Minute, ListAllProcess)
}

func ListAllProcess() {
	defer func() {
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
			log.Println(buffer.String())
		}
		defer time.AfterFunc(5*time.Minute, ListAllProcess)
	}()

	prs, e := ps.Processes()
	if nil != e {
		prs = nil
		log.Println("failed to list all processes,", e)
	}
	procs_lock.Lock()
	procs_list = prs
	procs_lock.Unlock()
}

func IsInProcessList(pid int, image string) bool {
	if processExists(pid, image) {
		return true
	}

	procs_lock.Lock()
	local_proces := procs_list
	procs_lock.Unlock()

	var pr ps.Process
	for _, p := range local_proces {
		if p.Pid() == pid {
			pr = p
			break
		}
	}
	if nil == pr {
		return false
	}

	if "" != image && !strings.Contains(strings.ToLower(pr.Executable()), strings.ToLower(image)) {
		return false
	}
	return true
}
