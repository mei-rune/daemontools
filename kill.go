package daemontools

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
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
