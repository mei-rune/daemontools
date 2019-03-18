package daemontools

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/rakyll/statik/fs"
	"github.com/runner-mei/cron"
	_ "github.com/runner-mei/daemontools/statik"
)

var (
	stop_by_id_list = []*regexp.Regexp{regexp.MustCompile(`^/?[^/]+/stop/?$`),
		regexp.MustCompile(`^/?daemons/[^/]+/stop/?$`),
		regexp.MustCompile(`^/?daemons/daemons/[^/]+/stop/?$`)}

	start_by_id_list = []*regexp.Regexp{regexp.MustCompile(`^/?[^/]+/start/?$`),
		regexp.MustCompile(`^/?daemons/[^/]+/start/?$`),
		regexp.MustCompile(`^/?daemons/daemons/[^/]+/start/?$`)}

	restart_by_id_list = []*regexp.Regexp{regexp.MustCompile(`^/?[^/]+/restart/?$`),
		regexp.MustCompile(`^/?daemons/[^/]+/restart/?$`),
		regexp.MustCompile(`^/?daemons/daemons/[^/]+/restart/?$`)}

	job_id_list = []*regexp.Regexp{regexp.MustCompile(`^/?[^/]+/?$`),
		regexp.MustCompile(`^/?daemons/[^/]+/?$`),
		regexp.MustCompile(`^/?daemons/daemons/[^/]+/?$`)}
)

type Manager struct {
	cr               *cron.Cron
	settings_file    string
	settings         map[string]interface{}
	supervisors      []supervisor
	pre_start_path   string
	post_finish_path string
	mode             string
	skipped          []string
	protected        []string
	fs               http.Handler
	on               func(string, int32)
}

func (self *Manager) On(on func(string, int32)) {
	self.on = on
}

func (self *Manager) onEvent(name string, status int32) {
	if self.on != nil {
		self.on(name, status)
	}
}

func (self *Manager) SetFs(fs http.Handler) {
	self.fs = fs
}

func (self *Manager) SetMode(mode string) {
	self.mode = mode
}

func (self *Manager) Names() []string {
	names := make([]string, len(self.supervisors))
	for idx, s := range self.supervisors {
		names[idx] = s.name()
	}
	return names
}

func (self *Manager) Mode() string {
	return self.mode
}

func (self *Manager) restart(name string) error {
	log.Println("[system] restart '" + name + "'")
	for _, sp := range self.supervisors {
		if sp.name() == name {
			sp.stop()
			sp.start()
			return nil
		}
	}
	log.Println("[system] kill '" + name + "'")
	return errors.New(name + " isn't found.")
}

func (self *Manager) start(name string) error {
	self.Enable(name)
	log.Println("[system] enable '" + name + "'")
	for _, sp := range self.supervisors {
		if sp.name() == name {
			sp.start()
			return nil
		}
	}
	return errors.New(name + " isn't found.")
}

func (self *Manager) stop(name string) error {
	self.Disable(name)

	log.Println("[system] disable '" + name + "'")
	for _, sp := range self.supervisors {
		if sp.name() == name {
			sp.stop()
			return nil
		}
	}
	return errors.New(name + " isn't found.")
}

func (self *Manager) Enable(name string) {
	skipped := make([]string, 0, len(self.skipped))
	for _, nm := range self.skipped {
		if nm == name {
			continue
		}
		skipped = append(skipped, nm)
	}
	self.skipped = skipped
}

func (self *Manager) Disable(name string) {
	self.skipped = append(self.skipped, name)
}

func (self *Manager) IsSipped(name string) bool {
	if 0 == len(self.skipped) {
		return false
	}
	for _, nm := range self.skipped {
		if nm == name {
			return true
		}
	}
	return false
}

func (self *Manager) EnableProtect(name string) {
	self.protected = append(self.protected, name)
}

func (self *Manager) DisableProtect(name string) {
	protected := make([]string, 0, len(self.protected))
	for _, nm := range self.protected {
		if nm == name {
			continue
		}
		protected = append(protected, nm)
	}
	self.protected = protected
}

func (self *Manager) IsProtected(name string) bool {
	if 0 == len(self.protected) {
		return false
	}
	for _, nm := range self.protected {
		if nm == name {
			return true
		}
	}

	return false
}

func (self *Manager) Stats() interface{} {
	res := make([]interface{}, 0, len(self.supervisors))
	for _, s := range self.supervisors {
		values := s.stats()
		if self.IsSipped(s.name()) {
			values["is_started"] = false
		}

		res = append(res, values)
	}
	return map[string]interface{}{"processes": res,
		"version":  "1.0",
		"settings": self.settings,
		"skipped":  self.skipped}
}

func (self *Manager) beforeStart() error {
	if FileExists(self.pre_start_path) {
		log.Println("execute '" + self.pre_start_path + "'")
		e := execute(self.pre_start_path)
		if nil != e {
			return errors.New("execute 'pre_start' failed, " + e.Error())
		}
	}
	return nil
}

func (self *Manager) afterStop() error {
	if FileExists(self.post_finish_path) {
		log.Println("execute '" + self.post_finish_path + "'")
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
	return self.Restore()
}

func (self *Manager) Restore() error {
	for _, s := range self.supervisors {
		if self.IsSipped(s.name()) {
			continue
		}
		if !s.isMode(self.mode) {
			continue
		}
		s.start()
	}

	var failed []string
	for _, s := range self.supervisors {
		if self.IsSipped(s.name()) {
			continue
		}

		if !s.isMode(self.mode) {
			continue
		}
		if e := s.untilStarted(); nil != e {
			failed = append(failed, fmt.Sprintf("start '%v' failed, %v", s.name(), e))
		}
	}
	if len(failed) > 0 {
		return errors.New(strings.Join(failed, "\r\n"))
	}
	return nil
}

func (self *Manager) Pause() error {
	return self.stopAll(false)
}

func (self *Manager) stopAll(all bool) error {
	var stopList []supervisor
	for _, s := range self.supervisors {
		//if self.IsSipped(s.name()) {
		//	continue
		//}

		if !all {
			if self.IsProtected(s.name()) {
				continue
			}
		}

		//if !s.isMode(self.mode) {
		//	continue
		//}
		s.stop()
		stopList = append(stopList, s)
	}

	var failed []string
	for _, s := range stopList {
		// if self.IsSipped(s.name()) {
		// 	continue
		// }
		// if !all {
		// 	fmt.Println("====", s.name(), "?")
		// 	if self.IsProtected(s.name()) {
		// 		continue
		// 	}
		// }
		// if !s.isMode(self.mode) {
		// 	continue
		// }

		if err := s.untilStopped(); nil != err {
			failed = append(failed, fmt.Sprintf("start '%v' failed, %v", s.name(), err))
		}
	}
	if len(failed) > 0 {
		return errors.New(strings.Join(failed, "\r\n"))
	}
	return nil
}

func (self *Manager) Stop() {
	if e := self.stopAll(true); nil != e {
		log.Println(e)
	}

	if e := self.afterStop(); nil != e {
		log.Println(e)
	}
}

func (self *Manager) RunForever(listenAddress string) {
	e := self.Start()
	if nil != e {
		goto end
	}
	http.Handle("/", self)
	log.Println("[daemontools] serving at '" + listenAddress + "'")
	if e := http.ListenAndServe(listenAddress, nil); nil != e {
		log.Println("[daemontools] fail to listen at '"+listenAddress+"'", e)
	}
	self.Stop()
end:
	if nil != e {
		log.Println("************************************************")
		log.Println(e)
	}
}

func (self *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.Method, r.URL.Path)
	switch r.Method {
	case "GET":
		if "/status" == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(self.Stats())
			return
		}

		if nil == self.fs {
			statikFS, err := fs.New()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, err.Error())
				return
			}
			//http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(statikFS)))
			self.fs = http.StripPrefix("/", http.FileServer(statikFS))
		}
		self.fs.ServeHTTP(w, r)
		return
	case "POST":
		ss := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if 2 == len(ss) {
			switch strings.ToLower(ss[1]) {
			case "restart":
				if e := self.restart(ss[0]); nil != e {
					w.WriteHeader(http.StatusInternalServerError)
					io.WriteString(w, e.Error())
				} else {
					w.WriteHeader(http.StatusOK)
					io.WriteString(w, "OK")
				}
				return
			case "start":
				if e := self.start(ss[0]); nil != e {
					w.WriteHeader(http.StatusInternalServerError)
					io.WriteString(w, e.Error())
				} else {
					w.WriteHeader(http.StatusOK)
					io.WriteString(w, "OK")
				}
				return
			case "stop":
				if e := self.stop(ss[0]); nil != e {
					w.WriteHeader(http.StatusInternalServerError)
					io.WriteString(w, e.Error())
				} else {
					w.WriteHeader(http.StatusOK)
					io.WriteString(w, "OK")
				}
				return
			}
		}
	}

	http.NotFound(w, r)
}
