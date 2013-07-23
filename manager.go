package main

import (
	"fmt"
	"os"
	"os/signal"
)

type manager struct {
	supervisors []supervisor
}

func (self *manager) rpel() {
	c := make(chan os.Signal, 1)
	exit := make(chan string, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	go func() {

	}()
	select {
	case s := <-c:
		fmt.Println("Got signal:", s)
		return
	case <-exit:
		return
	}
}

func (self *manager) run() {
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
