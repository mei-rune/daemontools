package daemontools

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"strings"
)

type Manager struct {
	settings_file    string
	settings         map[string]interface{}
	supervisors      []supervisor
	pre_start_path   string
	post_finish_path string
	mode             string
}

func (self *Manager) retry(name string) error {
	return errors.New("not implemented")
}

func (self *Manager) start(name string) error {
	return errors.New("not implemented")
}

func (self *Manager) stop(name string) error {
	return errors.New("not implemented")
}

func (self *Manager) Stats() interface{} {
	res := make([]interface{}, len(self.supervisors))
	for i, s := range self.supervisors {
		res[i] = s.stats()
	}
	return map[string]interface{}{"processes": res,
		"version":  "1.0",
		"settings": self.settings}
}

func (self *Manager) beforeStart() error {
	if FileExists(self.pre_start_path) {
		fmt.Println("execute '" + self.pre_start_path + "'")
		e := execute(self.pre_start_path)
		if nil != e {
			return errors.New("execute 'pre_start' failed, " + e.Error())
		}
	}
	return nil
}

func (self *Manager) afterStop() error {
	if FileExists(self.post_finish_path) {
		fmt.Println("execute '" + self.post_finish_path + "'")
		e := execute(self.post_finish_path)
		if nil != e {
			return errors.New("execute 'post_finish' failed, " + e.Error())
		}
	}
	return nil
}

func (self *Manager) Start() error {
	if e := self.beforeStart(); nil != e {
		return e
	}
	for _, s := range self.supervisors {
		if !s.isMode(self.mode) {
			continue
		}
		s.start()
	}

	for _, s := range self.supervisors {
		if !s.isMode(self.mode) {
			continue
		}
		if e := s.untilStarted(); nil != e {
			return fmt.Errorf("start '%v' failed, %v", s.name(), e)
		}
	}
	return nil
}

func (self *Manager) Stop() {
	for _, s := range self.supervisors {
		if !s.isMode(self.mode) {
			continue
		}
		s.stop()
	}

	for _, s := range self.supervisors {
		if !s.isMode(self.mode) {
			continue
		}

		if err := s.untilStopped(); nil != err {
			fmt.Println(fmt.Sprintf("stop '%v' failed, %v", s.name(), err))
		}
	}

	if e := self.afterStop(); nil != e {
		fmt.Println(e)
	}
}

func (self *Manager) RunForever() {
	e := self.Start()
	if nil != e {
		goto end
	}
	http.Handle("/", self)
	log.Println("[daemontools] serving at '" + *ListenAddress + "'")
	if e := http.ListenAndServe(*ListenAddress, nil); nil != e {
		log.Println("[daemontools] fail to listen at '"+*ListenAddress+"'", e)
	}
	self.Stop()
end:
	if nil != e {
		fmt.Println("************************************************")
		fmt.Println(e)
	}
}

func (self *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		switch r.URL.Path {
		case "/", "/index.html", "/index.htm", "/daemons", "/daemons/":
			indexHandler(w, r, self)
			return
		case "/static/daemons/bootstrap.css":
			bootstrapCssHandler(w, r)
			return
		case "/static/daemons/bootstrap_modal.js":
			bootstrapModalJsHandler(w, r)
			return
		case "/static/daemons/bootstrap_popover.js":
			bootstrapPopoverJsHandler(w, r)
			return
		case "/static/daemons/bootstrap_tab.js":
			bootstrapTabJsHandler(w, r)
			return
		case "/static/daemons/bootstrap_tooltip.js":
			bootstrapTooltipJsHandler(w, r)
			return
		case "/static/daemons/dj_mon.css":
			djmonCssHandler(w, r)
			return
		case "/static/daemons/dj_mon.js":
			djmonJsHandler(w, r)
			return
		case "/static/daemons/jquery.min.js":
			jqueryJsHandler(w, r)
			return
		case "/static/daemons/mustache.js":
			mustascheJsHandler(w, r)
			return
		}
	case "POST":
		for _, retry := range restart_by_id_list {
			if retry.MatchString(r.URL.Path) {
				ss := strings.Split(r.URL.Path, "/")
				e := self.retry(ss[len(ss)-2])
				if nil == e {
					indexHandlerWithMessage(w, r, self, "success", "The job has been queued for a re-run")
				} else {
					indexHandlerWithMessage(w, r, self, "error", e.Error())
				}
				return
			}
		}

		for _, retry := range start_by_id_list {
			if retry.MatchString(r.URL.Path) {
				ss := strings.Split(r.URL.Path, "/")
				e := self.start(ss[len(ss)-2])
				if nil == e {
					indexHandlerWithMessage(w, r, self, "success", "The job has been queued for a re-run")
				} else {
					indexHandlerWithMessage(w, r, self, "error", e.Error())
				}
				return
			}
		}

		for _, job_id := range stop_by_id_list {
			if job_id.MatchString(r.URL.Path) {
				ss := strings.Split(r.URL.Path, "/")
				e := self.stop(ss[len(ss)-2])
				if nil == e {
					indexHandlerWithMessage(w, r, self, "success", "The job was deleted")
				} else {
					indexHandlerWithMessage(w, r, self, "error", e.Error())
				}
				return
			}
		}
	}

	http.NotFound(w, r)
}
