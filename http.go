package daemontools

import (
	_ "expvar"
	"html/template"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"regexp"
)

var (
	cd_dir, _ = os.Getwd()

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

func fileHandler(w http.ResponseWriter, r *http.Request, path, default_content string) {
	var name string
	if filepath.IsAbs(path) {
		name = path
	} else {
		name = cd_dir + path
	}

	if fileExists(name) {
		http.ServeFile(w, r, name)
		return
	}

	io.WriteString(w, default_content)
}

func bootstrapCssHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/css; charset=utf-8"}
	fileHandler(w, r, "/static/daemons/bootstrap.css", bootstrap_css)
}
func bootstrapModalJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/daemons/bootstrap_modal.js", bootstrap_modal_js)
}
func bootstrapPopoverJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/daemons/bootstrap_popover.js", bootstrap_popover_js)
}
func bootstrapTabJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/daemons/bootstrap_tab.js", bootstrap_tab_js)
}
func bootstrapTooltipJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/daemons/bootstrap_tooltip.js", bootstrap_tooltip_js)
}
func djmonCssHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/css; charset=utf-8"}
	fileHandler(w, r, "/static/daemons/dj_mon.css", dj_mon_css)
}
func djmonJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/daemons/dj_mon.js", dj_mon_js)
}
func jqueryJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/daemons/jquery.min.js", jquery_min_js)
}
func mustascheJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/daemons/mustasche.js", mustasche_js)
}
func indexHandler(w http.ResponseWriter, r *http.Request, backend *Manager) {
	var e error
	var t *template.Template
	name := cd_dir + "/index.html"
	if fileExists(name) {
		t, e = template.ParseFiles(name)
	} else {
		t = template.New("default")
		t, e = t.Parse(index_html)
	}

	if nil != e {
		log.Println("[error]", e)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, e.Error())
		return
	}

	ctx := backend.Stats()
	if e = t.Execute(w, ctx); nil != e {
		log.Println("[error]", e)
	}
}
func indexHandlerWithMessage(w http.ResponseWriter, r *http.Request, backend *Manager, ok, message string) {
	indexHandler(w, r, backend)
}

// func allHandler(w http.ResponseWriter, r *http.Request, backend *Manager) {
// 	queryHandler(w, r, backend, nil)
// }

func httpServe(backend *Manager) {
	http.Handle("/", backend)

	log.Println("[daemontools] serving at '" + *listenAddress + "'")
	if e := http.ListenAndServe(*listenAddress, nil); nil != e {
		log.Println("[daemontools] fail to listen at '"+*listenAddress+"'", e)
	}
}
