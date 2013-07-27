package daemontools

import (
	_ "expvar"
	"html/template"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
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
	name := cd_dir + path
	if fileExist(name) {
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
func indexHandler(w http.ResponseWriter, r *http.Request, backend *manager) {
	var e error
	var t *template.Template
	name := cd_dir + "/index.html"
	if fileExist(name) {
		t, e = template.ParseFiles(name)
	} else {
		t = template.New("default")
		t, e = t.Parse(index_html)
	}

	if nil != e {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, e.Error())
		return
	}
	ctx := backend.Stats()
	t.Execute(w, ctx)
}

// func allHandler(w http.ResponseWriter, r *http.Request, backend *manager) {
// 	queryHandler(w, r, backend, nil)
// }

func httpServe(backend *manager) {
	http.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case "GET":
				switch r.URL.Path {
				case "/", "/index.html", "/index.htm", "/daemons", "/daemons/":
					indexHandler(w, r, backend)
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
				// for _, retry := range restart_by_id_list {
				// 	if retry.MatchString(r.URL.Path) {
				// 		ss := strings.Split(r.URL.Path, "/")
				// 		id, e := strconv.ParseInt(ss[len(ss)-2], 10, 0)
				// 		if nil != e {
				// 			indexHandlerWithMessage(w, r, "error", e.Error())
				// 			return
				// 		}

				// 		e = backend.retry(id)
				// 		if nil == e {
				// 			indexHandlerWithMessage(w, r, "success", "The job has been queued for a re-run")
				// 		} else {
				// 			indexHandlerWithMessage(w, r, "error", e.Error())
				// 		}
				// 		return
				// 	}
				// }

				// for _, retry := range start_by_id_list {
				// 	if retry.MatchString(r.URL.Path) {
				// 		ss := strings.Split(r.URL.Path, "/")
				// 		id, e := strconv.ParseInt(ss[len(ss)-2], 10, 0)
				// 		if nil != e {
				// 			indexHandlerWithMessage(w, r, "error", e.Error())
				// 			return
				// 		}

				// 		e = backend.retry(id)
				// 		if nil == e {
				// 			indexHandlerWithMessage(w, r, "success", "The job has been queued for a re-run")
				// 		} else {
				// 			indexHandlerWithMessage(w, r, "error", e.Error())
				// 		}
				// 		return
				// 	}
				// }

				// for _, job_id := range stop_by_id_list {
				// 	if job_id.MatchString(r.URL.Path) {
				// 		ss := strings.Split(r.URL.Path, "/")
				// 		id, e := strconv.ParseInt(ss[len(ss)-2], 10, 0)
				// 		if nil != e {
				// 			indexHandlerWithMessage(w, r, "error", e.Error())
				// 			return
				// 		}

				// 		e = backend.destroy(id)
				// 		if nil == e {
				// 			indexHandlerWithMessage(w, r, "success", "The job was deleted")
				// 		} else {
				// 			indexHandlerWithMessage(w, r, "error", e.Error())
				// 		}
				// 		return
				// 	}
				// }
			}

			w.WriteHeader(http.StatusNotFound)
		})
	log.Println("[daemontools] serving at '" + *listenAddress + "'")
	http.ListenAndServe(*listenAddress, nil)
}
