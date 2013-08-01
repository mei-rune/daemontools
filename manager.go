package daemontools

import (
	"fmt"
	"os"
	"os/signal"
)

type manager struct {
	supervisors []supervisor
}

func runCommand(s string) {
}

func (self *manager) Stats() interface{} {
	res := make([]interface{}, len(self.supervisors))
	for i, s := range self.supervisors {
		res[i] = s.stats()
	}
	return res
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
		//log.Println("[daemontools] serving at '" + *listenAddress + "'")
		//http.ListenAndServe(*listenAddress, nil)
		// reader := textproto.NewReader(bufio.NewReader(os.Stdin))
		// for {
		// 	s, e := reader.ReadLine()
		// 	if nil != e {
		// 		return
		// 	}

		// 	if "exit" == s {
		// 		exit <- s
		// 		break
		// 	}

		// 	runCommand(s)
		// }
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

	fmt.Println("[sys] start successfully.")

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
