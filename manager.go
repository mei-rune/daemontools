package daemontools

import (
	"bufio"
	"fmt"
	"net/textproto"
	"os"
	"os/signal"
)

type manager struct {
	supervisors []supervisor
}

func runCommand(s string) {
}

func (self *manager) rpel() {
	c := make(chan os.Signal, 1)
	exit := make(chan string, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	go func() {
		reader := textproto.NewReader(bufio.NewReader(os.Stdin))
		for {
			s, e := reader.ReadLine()
			if nil != e {
				return
			}

			if "exit" == s {
				exit <- s
				break
			}

			runCommand(s)
		}
	}()
	select {
	case s := <-c:
		fmt.Println("Got signal:", s)
		return
	case <-exit:
		return
	}
}

func (self *manager) runForever() {
	for _, s := range self.supervisors {
		s.Start()
	}

	var e error
	for _, s := range self.supervisors {
		e = s.UntilStarted()
		if nil != e {
			e = fmt.Errorf("start '%v' failed, %v", s.name, e)
			goto end
		}
	}

	self.rpel()

end:
	for _, s := range self.supervisors {
		s.Stop()
	}

	for _, s := range self.supervisors {
		err := s.UntilStopped()
		if nil != err {
			fmt.Println(fmt.Sprintf("stop '%v' failed, %v", s.name, err))
		}
	}

	if nil != e {
		fmt.Println("************************************************")
		fmt.Println(e)
	}
}
