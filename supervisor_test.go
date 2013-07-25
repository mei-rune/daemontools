package daemontools

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	prompt := "test_starts"
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: time.Second,
			//out:         os.Stdout,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), prompt}}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	e := s.untilStarted()
	if nil != e {
		t.Error(e)
	}
}

func TestStartWithoutPrompt(t *testing.T) {
	prompt := ""
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: time.Second,
			//out:         os.Stdout,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), "sdfsdfsdffsd"}}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	e := s.untilStarted()
	if nil != e {
		t.Error(e)
	}
}

func TestStartWithPrompt(t *testing.T) {
	prompt := "TestStartWithPrompt"
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: time.Second,
			//out:         os.Stdout,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), prompt}}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	e := s.untilStarted()
	if nil != e {
		t.Error(e)
	}
}

func TestStartWithRedirect(t *testing.T) {
	prompt := "TestStartWithRedirect"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: time.Second,
			out:         &buffer,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), prompt}}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	e := s.untilStarted()
	if nil != e {
		t.Error(e)
		t.Error(buffer.String())
	}
}

func TestStartWithEcho(t *testing.T) {
	prompt := "TestStartWithEcho"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: time.Second,
			out:         &buffer,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), prompt}}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	e := s.untilStarted()

	s.stop()
	s.untilStopped()

	ss := buffer.String()
	if nil != e {
		t.Error(e)
		t.Error(ss)
	} else if !strings.Contains(ss, prompt) {
		t.Error(ss)
	}
}

func TestStartFailed(t *testing.T) {
	prompt := "TestStartFailed"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: time.Second,
			out:         &buffer,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), "asdfsdf"}}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	ss := buffer.String()
	e := s.untilStarted()
	if nil == e {
		t.Error(ss)
	} else if strings.Contains(ss, prompt) {
		t.Error(ss)
	} else if strings.Contains(ss, "asdfsdf") {
		t.Error(ss)
	}
}

func TestStartFailedWithRepectedCount(t *testing.T) {
	count := int32(0)

	ln, e := net.Listen("tcp", ":0")
	if e != nil {
		fmt.Println()
		return
	}
	ar := strings.Split(ln.Addr().String(), ":")

	go func() {
		for {
			conn, e := ln.Accept()
			if e != nil {
				fmt.Println(e)
				break
			}
			atomic.AddInt32(&count, 1)
			reader := textproto.NewReader(bufio.NewReader(conn))
			_, e = reader.ReadLine()
			if nil != e {
				fmt.Println("c:", e)
			}
		}
	}()

	prompt := "TestStartFailedWithRepectedCount"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: time.Second,
			out:         &buffer,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "client.go"), "127.0.0.1:" + ar[len(ar)-1], "exit"}}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	e = s.untilStarted()
	ss := buffer.String()
	if nil == e {
		t.Error(ss)
	} else if strings.Contains(ss, prompt) {
		t.Error(ss)
	} else if s.repected != int(atomic.LoadInt32(&count)) {
		t.Errorf("restart count  '%d' is not 5", atomic.LoadInt32(&count))
		t.Error(ss)
	}
	defer ln.Close()
}

func TestStopByCmd(t *testing.T) {
	port := ":9483"
	prompt := "listen ok"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: 3 * time.Second,
			out:         &buffer,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "server.go"), port}},
			stop_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "client.go"), "127.0.0.1" + port, "exit"}}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	e := s.untilStarted()
	if nil != e {
		t.Error(e)
		return
	}

	s.stop()
	e = s.untilStopped()

	ss := buffer.String()

	if nil != e {
		t.Error(e)
		t.Error(ss)
	}

	if !strings.Contains(ss, "exit listen") {
		t.Error(ss)
	}
}

func TestStopByNoStop(t *testing.T) {
	prompt := "ok"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: 3 * time.Second,
			out:         &buffer,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "echo.go"), "TestStopByConsole"}}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	e := s.untilStarted()
	if nil != e {
		t.Error(e)
		return
	}

	s.stop()
	e = s.untilStopped()

	ss := buffer.String()

	if nil != e {
		t.Error(e)
		t.Error(ss)
	}

	if !strings.Contains(ss, "kill") {
		t.Error(ss)
	}
}

func TestStopByConsole(t *testing.T) {
	prompt := "ok"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: 3 * time.Second,
			out:         &buffer,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "echo.go"), "TestStopByConsole"}},
			stop_cmd: &command{proc: "__console__",
				arguments: []string{"TestsssStopByConsole2", "TestStopByConsole"}}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	e := s.untilStarted()
	if nil != e {
		t.Error(e)
		return
	}

	s.stop()
	e = s.untilStopped()

	ss := buffer.String()

	if nil != e {
		t.Error(e)
		t.Error(ss)
	}

	if !strings.Contains(ss, "TestsssStopByConsole2") {
		t.Error(ss)
	}

	if !strings.Contains(ss, "TestStopByConsole") {
		t.Error(ss)
	}
}

func TestStopByConsoleWithErrorExec(t *testing.T) {
	prompt := "ok"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: 3 * time.Second,
			out:         &buffer,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "echo.go"), "TestStopByConsole"}},
			stop_cmd: &command{proc: "__console__",
				arguments: []string{"TestStopByConsoleWithErrorExec", "TestStopByConsoleWithErrorExec"}}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	e := s.untilStarted()
	if nil != e {
		t.Error(e)
		return
	}

	s.stop()
	e = s.untilStopped()

	ss := buffer.String()

	if nil != e {
		t.Error(e)
		t.Error(ss)
	}

	if !strings.Contains(ss, "timed out after ") {
		t.Error(ss)
	}
}

func TestStopByKill(t *testing.T) {
	prompt := "ok"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor_default{prompt: prompt,
		supervisorBase: supervisorBase{proc_name: "test_start",
			repected:    5,
			killTimeout: 3 * time.Second,
			out:         &buffer,
			start_cmd: &command{proc: "go",
				arguments: []string{"run", filepath.Join(wd, "mock", "echo.go"), "TestStopByKill"}},
			stop_cmd: &command{proc: "__kill__"}}}

	s.start()

	defer func() {
		s.stop()
		s.untilStopped()
	}()

	e := s.untilStarted()
	if nil != e {
		t.Error(e)
		return
	}

	s.stop()
	e = s.untilStopped()

	ss := buffer.String()

	if nil != e {
		t.Error(e)
		t.Error(ss)
	}

	if !strings.Contains(ss, "kill") {
		t.Error(ss)
	}

	if strings.Contains(ss, "timed out after ") {
		t.Error(ss)
	}
}
