package daemontools

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"
)

func killByPid(pid int) error {
	if "windows" == runtime.GOOS {
		return killProcess(pid)
	} else {
		pr, e := os.FindProcess(pid)
		if nil != e {
			return e
		}
		defer pr.Release()
		return pr.Kill()
	}
}
func killProcessAndChildren(pid int, ps map[int]int) error {
	if nil == ps {
		var e error
		ps, e = enumProcesses()
		if nil != e {
			log.Println("killProcessAndChildren()" + e.Error())
			return killByPid(pid)
		}
	}

	for c, p := range ps {
		if p == pid {
			killProcessAndChildren(c, ps)
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

func pidExists(pid int) bool {
	pids, e := enumProcesses()
	if nil != e {
		return processExists(pid)
	}
	if _, ok := pids[pid]; ok {
		return true
	}
	return false
}

func processExists(pid int) bool {
	p, e := os.FindProcess(pid)
	if nil != e {
		if os.IsPermission(e) {
			return true
		}
		return false
	}
	p.Release()
	return true
}
