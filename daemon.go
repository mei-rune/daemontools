package daemontools

import (
	"bytes"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/runner-mei/cron"
)

var (
	RootDir     = "."
	LogDir      = ""
	Program     = "daemon"
	is_print    = flag.Bool("print", false, "print search paths while config is not found")
	config_file = flag.String("config", "", "the config file path")
	pre_start   = flag.String("pre_start", "pre_start.bat", "the name of pre start")
	post_finish = flag.String("post_finish", "post_finish.bat", "the name of post finish")
	JavaPath    = flag.String("JavaPath", "", "the path of java, should auto search if it is empty")
	Java15Path  = flag.String("Java15Path", "", "the path of java, should auto search if it is empty")
	mode        = flag.String("mode", "", "the mode of running")

	manager_exporter = &Exporter{}
)

func init() {
	flag.StringVar(&RootDir, "root", ".", "the root directory")
	Program = filepath.Base(os.Args[0])
}

func FileExists(nm string) bool {
	fs, e := os.Stat(nm)
	if nil != e {
		return false
	}
	return !fs.IsDir()
}

func DirExists(nm string) bool {
	fs, e := os.Stat(nm)
	if nil != e {
		return false
	}
	return fs.IsDir()
}

func abs(s string) string {
	r, e := filepath.Abs(s)
	if nil != e {
		return s
	}
	return r
}

func Init() error {
	Program = filepath.Base(os.Args[0])
	if !isPidInitialize() {
		if "windows" == runtime.GOOS {
			flag.Set("pid_file", Program+".pid")
		} else {
			flag.Set("pid_file", "/var/run/"+Program+".pid")
		}
	}

	if "." == RootDir {
		RootDir = abs(filepath.Dir(os.Args[0]))
		dirs := []string{abs(filepath.Dir(os.Args[0])), filepath.Join(abs(filepath.Dir(os.Args[0])), "..")}
		for _, s := range dirs {
			if DirExists(filepath.Join(s, "conf")) {
				RootDir = s
				break
			}
		}
	} else {
		RootDir = abs(RootDir)
	}

	if !DirExists(RootDir) {
		return errors.New("root directory '" + RootDir + "' is not exist.")
	} else {
		log.Println("root directory is '" + RootDir + "'.")
	}

	e := os.Chdir(RootDir)
	if nil != e {
		log.Println("change current dir to \""+RootDir+"\",", e)
	}
	return nil
}

func New(arguments map[string]interface{}) (*Manager, error) {
	guess_files := []string{filepath.Clean(abs(filepath.Join(RootDir, Program+".properties"))),
		filepath.Clean(abs(filepath.Join(RootDir, "etc", Program+".properties"))),
		filepath.Clean(abs(filepath.Join(RootDir, "conf", Program+".properties"))),
		filepath.Clean(abs(filepath.Join(RootDir, "daemon.properties"))),
		filepath.Clean(abs(filepath.Join(RootDir, "etc", "daemon.properties"))),
		filepath.Clean(abs(filepath.Join(RootDir, "conf", "daemon.properties"))),
		filepath.Clean(abs(filepath.Join(RootDir, "data", "etc", Program+".properties"))),
		filepath.Clean(abs(filepath.Join(RootDir, "data", "conf", Program+".properties"))),
		filepath.Clean(abs(filepath.Join(RootDir, "data", "etc", "daemon.properties"))),
		filepath.Clean(abs(filepath.Join(RootDir, "data", "conf", "daemon.properties"))),
		filepath.Clean(abs(filepath.Join("/etc", "hengwei", "daemon.properties"))),
		filepath.Clean(abs(filepath.Join("/etc", "tpt", "daemon.properties")))}

	var files []string
	for _, file := range guess_files {
		if FileExists(file) {
			files = append(files, file)
		}
	}

	if "" != *config_file {
		file := filepath.Clean(abs(*config_file))
		if !FileExists(file) {
			return nil, errors.New("config '" + file + "' is not exists.")
		}
		files = append(files, file)
	} else if len(files) <= 0 && *is_print {
		log.Println("config file is not found:")
		for _, nm := range guess_files {
			log.Println("    ", nm)
		}
	}

	if 0 == len(*JavaPath) {
		*JavaPath = search_java_home(RootDir)
		log.Println("[warn] java is", *JavaPath)
	}

	if 0 == len(*Java15Path) {
		*Java15Path = search_java15_home(RootDir)
		log.Println("[warn] java15 is", *Java15Path)
	}

	mgr, e := loadConfigs(RootDir, Program, files, arguments)
	if nil != e {
		return nil, e
	}

	if len(mgr.settings) > 0 {
		for k, s := range mgr.settings {
			if strings.HasSuffix(k, ".disabled") {
				name := strings.TrimSpace(strings.TrimSuffix(k, ".disabled"))
				switch strings.ToLower(fmt.Sprint(s)) {
				case "1", "yes", "true":
					mgr.Disable(name)
				case "0", "no", "false":
					mgr.Enable(name)
				default:
					log.Println("'" + k + "=" + fmt.Sprint(s) + "' is invalid.")
					os.Exit(1)
				}
			}
		}
	}

	expvar.Publish("supervisors", manager_exporter)
	manager_exporter.Var = mgr

	pre_start_path := filepath.Join(RootDir, *pre_start)
	if "pre_start.bat" == *pre_start {
		if "windows" != runtime.GOOS {
			pre_start_path = filepath.Join(RootDir, "pre_start.sh")
		}
	}

	post_finish_path := filepath.Join(RootDir, *post_finish)
	if "post_finish.bat" == *post_finish {
		if "windows" != runtime.GOOS {
			post_finish_path = filepath.Join(RootDir, "post_finish.sh")
		}
	}

	mgr.pre_start_path = pre_start_path
	mgr.post_finish_path = post_finish_path
	return mgr, nil
}

func search_java_home(root string) string {
	return search_java_home_with_version(root, "")
}

func search_java15_home(root string) string {
	return search_java_home_with_version(root, "15")
}

func search_java_home_with_version(root, version string) string {
	java_execute := "java.exe"
	if "windows" != runtime.GOOS {
		java_execute = "java"
	}

	jp := filepath.Join(root, "runtime_env/jdk"+version+"/bin", java_execute)
	if FileExists(jp) {
		return jp
	}

	jp = filepath.Join(root, "runtime_env/java"+version+"/bin", java_execute)
	if FileExists(jp) {
		return jp
	}

	jp = filepath.Join(root, "runtime_env/jre"+version+"/bin", java_execute)
	if FileExists(jp) {
		return jp
	}

	ss, _ := filepath.Glob(filepath.Join(root, "**", "java.exe"))
	if nil != ss && 0 != len(ss) {
		return ss[0]
	}

	jh := os.Getenv("JAVA" + version + "_HOME")
	if "" != jh {
		return filepath.Join(jh, "bin", java_execute)
	}

	return java_execute
}

func execute(pa string) error {
	cmd := exec.Command(pa)
	os_env := os.Environ()
	environments := make([]string, 0, 1+len(os_env))
	environments = append(environments, os_env...)
	environments = append(environments, "PROCMGR_ID="+os.Args[0])
	cmd.Env = environments
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	log.Println("===================== output begin =====================")
	defer func() {
		log.Println("=====================  output end  =====================")
	}()
	return cmd.Run()
}

func loadConfigs(root, execute string, files []string, defaultArgs map[string]interface{}) (*Manager, error) {
	var arguments map[string]interface{}
	//"autostart_"
	if len(files) > 0 {
		var e error
		arguments, e = loadProperties(root, files)
		if nil != e {
			return nil, e
		}
	} else {
		log.Println("[warn] the daemon config file is not found.")
	}

	if nil == arguments {
		arguments = loadDefault(root, "")
	}

	if "" == execute {
		arguments["execute"] = "daemontools"
	} else {
		arguments["execute"] = execute
	}

	for k, v := range defaultArgs {
		arguments[k] = v
	}

	patterns := stringsWithDefault(arguments, "patterns", ";", nil)
	patterns = append(patterns, filepath.Clean(abs(filepath.Join(root, "autostart_*.conf"))),
		filepath.Clean(abs(filepath.Join(root, "*_autostart.conf"))),
		filepath.Clean(abs(filepath.Join(root, "*-autostart.conf"))),
		filepath.Clean(abs(filepath.Join(root, "autostart-*.conf"))),
		filepath.Clean(abs(filepath.Join(root, "*/*_autostart.conf"))),
		filepath.Clean(abs(filepath.Join(root, "*/*-autostart.conf"))),
		filepath.Clean(abs(filepath.Join(root, "*/autostart-*.conf"))),
		filepath.Clean(abs(filepath.Join(root, "*/autostart_*.conf"))))

	mgr := &Manager{
		cr: cron.New(),
	}
	supervisors := make([]supervisor, 0, 10)
	for _, pattern := range patterns {
		matches, e := filepath.Glob(pattern)
		if nil != e {
			return nil, errors.New("search '" + pattern + "' failed, " + e.Error())
		}

		if nil == matches {
			continue
		}

		for _, nm := range matches {
			supervisors, e = loadConfig(nm, arguments, mgr.onEvent, supervisors)
			if nil != e {
				return nil, errors.New("load '" + nm + "' failed, " + e.Error())
			}
			log.Println("load '" + nm + "' is ok.")
		}
	}

	if LogDir == "" {
		logPath := filepath.Clean(abs(filepath.Join(root, "logs")))
		logs := []string{stringWithDefault(arguments, "logPath", logPath),
			filepath.Clean(abs(filepath.Join(root, "..", "logs"))),
			logPath}

		for _, s := range logs {
			if DirExists(s) {
				logPath = s
				break
			}
		}
		LogDir = logPath
	}

	if !DirExists(LogDir) {
		os.Mkdir(LogDir, 0660)
	}

	logArguments := mapWithDefault(arguments, "log", map[string]interface{}{})
	maxBytes := intWithDefault(logArguments, "maxBytes", 0)
	maxNum := intWithDefault(logArguments, "maxNum", 0)
	if maxBytes < 1*1024*1024 {
		maxBytes = 5 * 1024 * 1024
	}
	if maxNum <= 0 {
		maxNum = 5
	}

	for _, s := range supervisors {
		out, e := NewRotateFile(filepath.Clean(abs(filepath.Join(LogDir, s.name()+".log"))), maxBytes, maxNum)
		if nil != e {
			return nil, errors.New("open log failed for '" + s.name() + "', " + e.Error())
		}
		s.setOutput(out)
	}

	file := filepath.Join(RootDir, "data", "conf", "daemon.properties")
	if len(files) > 0 && (strings.HasPrefix(files[len(files)-1], "/var/") ||
		strings.Contains(files[len(files)-1], "/data/conf/") ||
		strings.Contains(files[len(files)-1], "\\data\\conf\\")) {
		file = files[len(files)-1]
	}

	mgr.settings = arguments
	mgr.settings_file = file
	mgr.supervisors = supervisors
	for idx := range mgr.supervisors {
		mgr.supervisors[idx].setManager(mgr)
	}
	mgr.cr.Start()
	return mgr, nil
}

func loadConfig(file string, args map[string]interface{}, on func(string, int32), supervisors []supervisor) ([]supervisor, error) {
	t, e := loadTemplateFile(file)
	if nil != e {
		return nil, errors.New("read file failed, " + e.Error())
	}

	args["cd_dir"] = filepath.Dir(file)

	var buffer bytes.Buffer
	e = t.Execute(&buffer, args)
	if nil != e {
		return nil, errors.New("regenerate file failed, " + e.Error())
	}

	var v interface{}
	e = Unmarshal(buffer.Bytes(), &v)
	//var attributes map[string]interface{}
	//e = json.Unmarshal(buffer.Bytes(), &attributes)
	if nil != e {
		log.Println(buffer.String())
		return nil, errors.New("ummarshal file failed, " + e.Error())
	}
	switch value := v.(type) {
	case map[string]interface{}:
		arguments := []map[string]interface{}{value, args}
		return loadSupervisor(file, arguments, on, supervisors)
	case []interface{}:
		for idx, o := range value {
			attributes, ok := o.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("[%v] is not a map or array.", idx)
			}

			arguments := []map[string]interface{}{attributes, args}
			supervisors, e = loadSupervisor(file, arguments, on, supervisors)
			if nil != e {
				return nil, e
			}
		}
		return supervisors, nil
	}
	return nil, fmt.Errorf("it is not a map or array - %T", v)
}

func loadSupervisor(file string, arguments []map[string]interface{}, on func(string, int32), supervisors []supervisor) ([]supervisor, error) {
	// type supervisor struct {
	//   name              string
	//   success_flag      string
	//   retries           int
	//   killTimeout       time.Duration
	//   start             *command
	//   stop              *command
	// }

	name := stringWithArguments(arguments, "name", "")
	if 0 == len(name) {
		return nil, errors.New("'name' is missing.")
	}
	retries := intWithArguments(arguments, "retries", 5)
	if retries <= 0 {
		return nil, errors.New("'retries' must is greate 0.")
	}
	killTimeout := durationWithArguments(arguments, "killTimeout", 5*time.Second)
	if killTimeout <= 0*time.Second {
		return nil, errors.New("'killTimeout' must is greate 0s.")
	}
	var start *command = nil
	var stop *command = nil

	o, ok := arguments[0]["start"]
	if !ok {
		return nil, errors.New("'start' is missing.")
	}

	m, ok := o.(map[string]interface{})
	if !ok {
		return nil, errors.New("'start' is invalid.")
	}

	start, e := loadCommand(append([]map[string]interface{}{m}, arguments[1:]...))
	if nil != e {
		return nil, e
	}

	o, ok = arguments[0]["stop"]
	if ok {
		m, ok = o.(map[string]interface{})
		if !ok {
			return nil, errors.New("'stop' is invalid.")
		}

		stop, e = loadCommand(append([]map[string]interface{}{m}, arguments[1:]...))
		if nil != e {
			return nil, e
		}
	}

	successFlag := stringWithArguments(arguments, "success_flag", "")
	if 0 == len(successFlag) {
		retries1 := intWithDefault(arguments[0], "retries", 0)
		if retries1 > 0 {
			log.Println("[warn] retries will ignore while success_flag is missing in '" + name + "' at '" + file + "'.")
		}
	}

	for _, sp := range supervisors {
		if sp.name() == name {
			return nil, errors.New("'" + name + "' of '" + file + "' is already exists in the " + sp.fileName())
		}
	}

	restartSchedule := stringWithArguments(arguments, "restart_schedule", "")
	pidfile := stringWithArguments(arguments, "pidfile", "")
	if 0 != len(pidfile) {
		if nil != stop {
			switch stop.proc {
			case "__kill___", "":
				return nil, errors.New("'__kill___' is not unsupported for pidfile")
			case "__console__":
				return nil, errors.New("'__console__' is not unsupported for pidfile")
			}
		}

		pidfile = filepath.Clean(abs(pidfile))
		supervisors = append(supervisors, &supervisorWithPidfile{pidfile: pidfile,
			supervisorBase: supervisorBase{
				file:            file,
				proc_name:       name,
				restartSchedule: restartSchedule,
				mode:            stringWithArguments(arguments, "mode", ""),
				retries:         retries,
				killTimeout:     killTimeout,
				on:              on,
				start_cmd:       start,
				stop_cmd:        stop}})

	} else {
		supervisors = append(supervisors, &supervisor_default{success_flag: successFlag,
			supervisorBase: supervisorBase{
				file:            file,
				proc_name:       name,
				restartSchedule: restartSchedule,
				mode:            stringWithArguments(arguments, "mode", ""),
				retries:         retries,
				killTimeout:     killTimeout,
				on:              on,
				start_cmd:       start,
				stop_cmd:        stop}})
	}
	return supervisors, nil
}

func loadCommand(args []map[string]interface{}) (*command, error) {
	//   type command struct {
	//   proc         string
	//   arguments    []string
	//   environments []string
	//   directory    string
	// }
	proc := stringWithArguments(args, "execute", "")
	if 0 == len(proc) {
		return nil, errors.New("'execute' is missing.")
	}

	arguments := stringsWithArguments(args, "arguments", "", nil, false)
	environments := stringsWithArguments(args, "environments", "", nil, false)
	directory := stringWithDefault(args[0], "directory", "")
	if 0 == len(directory) && 1 < len(args) {
		directory = stringWithArguments(args[1:], "root_dir", "")
	}

	switch strings.ToLower(filepath.Base(proc)) {
	case "java", "java.exe":
		var e error
		arguments, e = loadJavaArguments(arguments, args)
		if nil != e {
			return nil, e
		}

		if "java" == proc || "java.exe" == proc {
			version := stringWithArguments(args, "java_version", "")
			if version == "15" {
				proc = stringWithArguments(args, "java", *Java15Path)
			} else {
				proc = stringWithArguments(args, "java", *JavaPath)
			}
		}

	case "java15", "java15.exe":
		var e error
		arguments, e = loadJavaArguments(arguments, args)
		if nil != e {
			return nil, e
		}

		if "java15" == proc || "java15.exe" == proc {
			proc = stringWithArguments(args, "java15", *Java15Path)
		}
	}

	return &command{proc: proc, arguments: arguments, environments: environments, directory: directory}, nil
}

func loadJavaArguments(arguments []string, args []map[string]interface{}) ([]string, error) {
	var results []string

	java_ms := stringWithArguments(args, "java_mem_mix", "")
	if "" != java_ms {
		results = append(results, "-Xms"+java_ms)
	}
	java_mx := stringWithArguments(args, "java_mem_max", "")
	if "" != java_mx {
		results = append(results, "-Xmx"+java_mx)
	}

	cp := stringsWithArguments(args, "java_classpath", ";", nil, false)
	if nil != cp && 0 != len(cp) {
		var classpath []string
		for _, p := range cp {
			if 0 == len(p) {
				continue
			}
			files, e := filepath.Glob(p)
			if nil != e {
				return nil, e
			}
			if nil == files {
				continue
			}

			classpath = append(classpath, files...)
		}

		if nil != classpath && 0 != len(classpath) {
			if "windows" == runtime.GOOS {
				results = append(results, "-cp", strings.Join(classpath, ";"))
			} else {
				results = append(results, "-cp", strings.Join(classpath, ":"))
			}
		}
	}

	debug := stringWithArguments(args, "java_debug", "")
	if 0 != len(debug) {
		suspend := boolWithArguments(args, "java_debug_suspend", false)
		if suspend {
			results = append(results, "-agentlib:jdwp=transport=dt_socket,server=n,suspend=y,address="+debug)
		} else {
			results = append(results, "-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address="+debug)
		}
	}

	// JAVA_ARGS="${JAVA_ARGS} -Dcom.sun.management.jmxremote -Dcom.sun.management.jmxremote.port=18889 -Dcom.sun.management.jmxremote.authenticate=true -Dcom.sun.management.jmxremote.ssl=false"
	// JAVA_ARGS="${JAVA_ARGS} -Dcom.sun.management.jmxremote.access.file=/tmp/jmx.access"
	// JAVA_ARGS="${JAVA_ARGS} -Dcom.sun.management.jmxremote.password.file=/tmp/jmx.pass"
	jmx_option := stringWithArguments(args, "jmx_option", "")
	if "true" == jmx_option || "enable" == jmx_option {
		results = append(results, "-Dcom.sun.management.jmxremote")
		if jmx_port := stringWithArguments(args, "jmx_port", ""); "" != jmx_port {
			results = append(results, "-Dcom.sun.management.jmxremote.port="+jmx_port)
		}

		jmx_password := stringWithArguments(args, "jmx_password", "")
		jmx_access := stringWithArguments(args, "jmx_access", "")
		if "" != jmx_access && "" != jmx_password {
			results = append(results, "-Dcom.sun.management.jmxremote.authenticate=true")
			results = append(results, "-Dcom.sun.management.jmxremote.access.file="+jmx_access)
			results = append(results, "-Dcom.sun.management.jmxremote.password.file="+jmx_password)
		} else {
			results = append(results, "-Dcom.sun.management.jmxremote.authenticate=false")
		}
		results = append(results, "-Dcom.sun.management.jmxremote.ssl=false")
		results = append(results, "-Dcom.sun.management.jmxremote.local.only=false")
	}

	options := stringsWithArguments(args, "java_options", ",", nil, false)
	if nil != options && 0 != len(options) {
		results = append(results, options...)
	}

	class := stringWithArguments(args, "java_class", "")
	if 0 != len(class) {
		results = append(results, class)
	}

	jar := stringWithArguments(args, "java_jar", "")
	if 0 != len(jar) {
		results = append(results, jar)
	}

	if nil != arguments && 0 != len(arguments) {
		return append(results, arguments...), nil
	}
	return results, nil
}

func loadDefault(root, file string) map[string]interface{} {
	file_dir := ""
	if "" != file {
		file_dir = filepath.Dir(file)
	}

	return map[string]interface{}{"root_dir": root,
		"file_dir": file_dir,
		"java15":   *Java15Path,
		"java":     *JavaPath,
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH}
}

func loadProperties(root string, files []string) (map[string]interface{}, error) {

	osExt := ".exe"
	batExt := ".bat"
	if runtime.GOOS != "windows" {
		osExt = ""
		batExt = ".sh"
	}

	var all_arguments = make(map[string]interface{})
	for _, file := range files {
		t, e := loadTemplateFile(file)
		if nil != e {
			return nil, errors.New("read config '" + file + "' failed, " + e.Error())
		}
		args := loadDefault(root, file)

		args["sh_ext"] = batExt
		args["os_ext"] = osExt
		var buffer bytes.Buffer
		e = t.Execute(&buffer, args)
		if nil != e {
			return nil, errors.New("generate config '" + file + "' failed, " + e.Error())
		}

		var arguments = readProperties(&buffer)
		if len(arguments) > 0 {
			for k, v := range arguments {
				all_arguments[k] = v
			}
		}
	}

	if s, ok := all_arguments["java"]; ok {
		*JavaPath = fmt.Sprint(s)
	} else {
		all_arguments["java"] = *JavaPath
	}

	if s, ok := all_arguments["java15"]; ok {
		*Java15Path = fmt.Sprint(s)
	} else {
		all_arguments["java15"] = *Java15Path
	}

	all_arguments["root_dir"] = root
	all_arguments["os"] = runtime.GOOS
	all_arguments["os_ext"] = osExt
	all_arguments["sh_ext"] = batExt
	all_arguments["arch"] = runtime.GOARCH
	return all_arguments, nil
}

var funcs = template.FuncMap{
	"fileExists":   FileExists,
	"joinFilePath": filepath.Join,
	"joinUrlPath": func(base string, paths ...string) string {
		var buf bytes.Buffer
		buf.WriteString(base)

		lastSplash := strings.HasSuffix(base, "/")
		for _, pa := range paths {
			if 0 == len(pa) {
				continue
			}

			if lastSplash {
				if '/' == pa[0] {
					buf.WriteString(pa[1:])
				} else {
					buf.WriteString(pa)
				}
			} else {
				if '/' != pa[0] {
					buf.WriteString("/")
				}
				buf.WriteString(pa)
			}

			lastSplash = strings.HasSuffix(pa, "/")
		}
		return buf.String()
	},
}

func loadTemplateFile(file string) (*template.Template, error) {
	bs, e := ioutil.ReadFile(file)
	if nil != e {
		return nil, errors.New("read file failed, " + e.Error())
	}
	return template.New("default").Funcs(funcs).Parse(string(bs))
}
