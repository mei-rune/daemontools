package daemontools

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
)

type manager struct {
	settings_file string
	settings      map[string]interface{}
	supervisors   []supervisor
}

func (self *manager) retry(name string) error {
	return errors.New("not implemented")
}

func (self *manager) start(name string) error {
	return errors.New("not implemented")
}

func (self *manager) stop(name string) error {
	return errors.New("not implemented")
}

func (self *manager) Stats() interface{} {
	res := make([]interface{}, len(self.supervisors))
	for i, s := range self.supervisors {
		res[i] = s.stats()
	}
	return map[string]interface{}{"processes": res,
		"version":  "1.0",
		"settings": self.settings}
}

func (self *manager) rpel() {
	c := make(chan os.Signal, 1)
	exit := make(chan string, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	select {
	case s := <-c:
		fmt.Println("Got signal:", s)
		return
	case <-exit:
		return
	}
}

func (self *manager) runForever() {
	go func() {
		httpServe(self)
	}()

	for _, s := range self.supervisors {
		s.start()
	}

	var e error
	for _, s := range self.supervisors {
		e = s.untilStarted()
		if nil != e {
			e = fmt.Errorf("start '%v' failed, %v", s.name(), e)
			goto end
		}
	}

	self.rpel()

end:
	for _, s := range self.supervisors {
		s.stop()
	}

	for _, s := range self.supervisors {
		err := s.untilStopped()
		if nil != err {
			fmt.Println(fmt.Sprintf("stop '%v' failed, %v", s.name(), err))
		}
	}

	if nil != e {
		fmt.Println("************************************************")
		fmt.Println(e)
	}
}
