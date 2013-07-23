package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestKillProcess(t *testing.T) {
	pr := exec.Command("ping", "127.0.0.1", "-t")
	e := pr.Start()
	if nil != e {
		t.Error(e)
		return
	}
	kp, _ := os.FindProcess(pr.Process.Pid)
	e = kp.Kill()

	if nil != e {
		t.Error(e)
		pr.Process.Kill()
		return
	}
}
