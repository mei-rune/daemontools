package daemontools

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/mitchellh/go-ps"
)

var PidFile string

func init() {
	if "windows" == runtime.GOOS {
		flag.StringVar(&PidFile, "pid_file", "daemontools.pid", "File containing process PID")
	} else {
		flag.StringVar(&PidFile, "pid_file", "/var/run/daemontools.pid", "File containing process PID")
	}
}

func isPidInitialize() bool {
	ret := false
	flag.Visit(func(f *flag.Flag) {
		if "pid_file" == f.Name {
			ret = true
		}
	})
	return ret
}

func CreatePidFile(pidFile, image string) error {
	if pidString, err := ioutil.ReadFile(pidFile); err == nil {
		pid, err := strconv.Atoi(string(pidString))
		if err == nil {
			if pr, e := ps.FindProcess(pid); nil != e || (nil != pr &&
				strings.Contains(strings.ToLower(pr.Executable()), strings.ToLower(image))) {
				return fmt.Errorf("pid file is already exists, ensure "+image+" is not running or delete %s.", pidFile)
			}
		}
	}

	file, err := os.Create(pidFile)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = fmt.Fprintf(file, "%d", os.Getpid())
	return err
}

func RemovePidFile(pidFile string) {
	if err := os.Remove(pidFile); err != nil {
		fmt.Printf("Error removing %s: %s\r\n", pidFile, err)
	}
}
