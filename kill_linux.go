// +build darwin freebsd linux netbsd openbsd
package daemontools

import (
	"errors"
	"os"
)

func enumProcesses() (map[int]int, error) {
	return nil, errors.New("not implemented")
}

func killProcess(pid int) error {
	p, e := os.FindProcess(pid)
	if nil != e {
		return e
	}

	return p.Kill()
}
