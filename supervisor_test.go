package main

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
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: time.Second,
		//out:         os.Stdout,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), prompt}}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	e := s.UntilStarted()
	if nil != e {
		t.Error(e)
	}
}

func TestStartWithoutPrompt(t *testing.T) {
	prompt := ""
	wd, _ := os.Getwd()
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: time.Second,
		//out:         os.Stdout,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), "sdfsdfsdffsd"}}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	e := s.UntilStarted()
	if nil != e {
		t.Error(e)
	}
}

func TestStartWithPrompt(t *testing.T) {
	prompt := "TestStartWithPrompt"
	wd, _ := os.Getwd()
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: time.Second,
		//out:         os.Stdout,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), prompt}}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	e := s.UntilStarted()
	if nil != e {
		t.Error(e)
	}
}

func TestStartWithRedirect(t *testing.T) {
	prompt := "TestStartWithRedirect"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: time.Second,
		out:         &buffer,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), prompt}}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	e := s.UntilStarted()
	if nil != e {
		t.Error(e)
		t.Error(buffer.String())
	}
}

func TestStartWithEcho(t *testing.T) {
	prompt := "TestStartWithEcho"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: time.Second,
		out:         &buffer,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), prompt}}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	e := s.UntilStarted()

	s.Stop()
	s.UntilStopped()

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
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: time.Second,
		out:         &buffer,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "helloworld.go"), "asdfsdf"}}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	ss := buffer.String()
	e := s.UntilStarted()
	if nil == e {
		t.Error(ss)
	} else if strings.Contains(ss, prompt) {
		t.Error(ss)
	} else if strings.Contains(ss, "asdfsdf") {
		t.Error(ss)
	}
}

func TestStartFailedWithRepectedCount(t *testing.T) {
	port := ":9483"
	count := int32(0)

	ln, e := net.Listen("tcp", port)
	if e != nil {
		fmt.Println()
		return
	}

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
				fmt.Println(e)
				break
			}
		}
	}()

	prompt := "TestStartFailedWithRepectedCount"
	var buffer bytes.Buffer
	wd, _ := os.Getwd()
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: time.Second,
		out:         &buffer,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "client.go"), "127.0.0.1" + port, "exit"}}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	ss := buffer.String()
	e = s.UntilStarted()
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
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: 3 * time.Second,
		out:         &buffer,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "server.go"), port}},
		stop: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "client.go"), "127.0.0.1" + port, "exit"}}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	e := s.UntilStarted()
	if nil != e {
		t.Error(e)
		return
	}

	s.Stop()
	e = s.UntilStopped()

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
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: 3 * time.Second,
		out:         &buffer,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "echo.go"), "TestStopByConsole"}}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	e := s.UntilStarted()
	if nil != e {
		t.Error(e)
		return
	}

	s.Stop()
	e = s.UntilStopped()

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
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: 3 * time.Second,
		out:         &buffer,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "echo.go"), "TestStopByConsole"}},
		stop: &command{proc: "__console__",
			arguments: []string{"TestsssStopByConsole2", "TestStopByConsole"}}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	e := s.UntilStarted()
	if nil != e {
		t.Error(e)
		return
	}

	s.Stop()
	e = s.UntilStopped()

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
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: 3 * time.Second,
		out:         &buffer,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "echo.go"), "TestStopByConsole"}},
		stop: &command{proc: "__console__",
			arguments: []string{"TestStopByConsoleWithErrorExec", "TestStopByConsoleWithErrorExec"}}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	e := s.UntilStarted()
	if nil != e {
		t.Error(e)
		return
	}

	s.Stop()
	e = s.UntilStopped()

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
	s := &supervisor{name: "test_start",
		prompt:      prompt,
		repected:    5,
		killTimeout: 3 * time.Second,
		out:         &buffer,
		start: &command{proc: "go",
			arguments: []string{"run", filepath.Join(wd, "mock", "echo.go"), "TestStopByKill"}},
		stop: &command{proc: "__kill__"}}

	s.Start()

	defer func() {
		s.Stop()
		s.UntilStopped()
	}()

	e := s.UntilStarted()
	if nil != e {
		t.Error(e)
		return
	}

	s.Stop()
	e = s.UntilStopped()

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
