package daemontools

import (
	"bytes"
	"encoding/json"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"
)

var (
	is_print      = flag.Bool("print", false, "print search paths while config is not found")
	root_dir      = flag.String("root", ".", "the root directory")
	config_file   = flag.String("config", "./<program_name>.conf", "the config file path")
	listenAddress = flag.String("listen", ":37070", "the address of http")
	pre_start     = flag.String("pre_start", "pre_start.bat", "the name of pre start")
	post_finish   = flag.String("post_finish", "post_finish.bat", "the name of post finish")
	java_path     = flag.String("java_path", "", "the path of java, should auto search if it is empty")
	mode          = flag.String("mode", "", "the mode of running")

	manager_exporter = &Exporter{}
)

func fileExists(nm string) bool {
	fs, e := os.Stat(nm)
	if nil != e {
		return false
	}
	return !fs.IsDir()
}

func dirExists(nm string) bool {
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

func New() (*Manager, error) {
	nm := filepath.Base(os.Args[0])
	if !isPidInitialize() {
		if "windows" == runtime.GOOS {
			flag.Set("pid_file", nm+".pid")
		} else {
			flag.Set("pid_file", "/var/run/"+nm+".pid")
		}
	}

	if err := createPidFile(*pidFile, nm); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer removePidFile(*pidFile)

	if "." == *root_dir {
		*root_dir = abs(filepath.Dir(os.Args[0]))
		dirs := []string{abs(filepath.Dir(os.Args[0])), filepath.Join(abs(filepath.Dir(os.Args[0])), "..")}
		for _, s := range dirs {
			if dirExists(filepath.Join(s, "conf")) {
				*root_dir = s
				break
			}
		}
	} else {
		*root_dir = abs(*root_dir)
	}

	if !dirExists(*root_dir) {
		return nil, errors.New("root directory '" + *root_dir + "' is not exist.")
	} else {
		fmt.Println("root directory is '" + *root_dir + "'.")
	}

	e := os.Chdir(*root_dir)
	if nil != e {
		fmt.Println("change current dir to \"" + *root_dir + "\"")
	}

	file := ""
	if "" == *config_file || "./<program_name>.conf" == *config_file {
		program := filepath.Base(os.Args[0])
		files := []string{filepath.Clean(abs(filepath.Join(*root_dir, program+".conf"))),
			filepath.Clean(abs(filepath.Join(*root_dir, "etc", program+".conf"))),
			filepath.Clean(abs(filepath.Join(*root_dir, "conf", program+".conf"))),
			filepath.Clean(abs(filepath.Join(*root_dir, "daemon.conf"))),
			filepath.Clean(abs(filepath.Join(*root_dir, "etc", "daemon.conf"))),
			filepath.Clean(abs(filepath.Join(*root_dir, "conf", "daemon.conf"))),
			filepath.Clean(abs(filepath.Join(*root_dir, "data", "etc", program+".conf"))),
			filepath.Clean(abs(filepath.Join(*root_dir, "data", "conf", program+".conf"))),
			filepath.Clean(abs(filepath.Join(*root_dir, "data", "etc", "daemon.conf"))),
			filepath.Clean(abs(filepath.Join(*root_dir, "data", "conf", "daemon.conf")))}

		found := false
		for _, nm := range files {
			if fileExists(nm) {
				found = true
				file = nm
				break
			}
		}

		if !found && *is_print {
			fmt.Println("config file is not found:")
			for _, nm := range files {
				fmt.Println("    ", nm)
			}
		}
	} else {
		file = filepath.Clean(abs(*config_file))
		if !fileExists(file) {
			return nil, errors.New("config '" + file + "' is not exists.")

		}
	}

	if 0 == len(*java_path) {
		*java_path = search_java_home(*root_dir)
		fmt.Println("[warn] java is", *java_path)
	}

	mgr, e := loadConfigs(*root_dir, file, nm)
	if nil != e {
		return nil, e
	}

	expvar.Publish("supervisors", manager_exporter)
	manager_exporter.Var = mgr

	pre_start_path := filepath.Join(*root_dir, *pre_start)
	if "pre_start.bat" == *pre_start {
		if "windows" != runtime.GOOS {
			pre_start_path = filepath.Join(*root_dir, "pre_start.sh")
		}
	}

	post_finish_path := filepath.Join(*root_dir, *post_finish)
	if "post_finish.bat" == *post_finish {
		if "windows" != runtime.GOOS {
			post_finish_path = filepath.Join(*root_dir, "post_finish.sh")
		}
	}

	mgr.pre_start_path = pre_start_path
	mgr.post_finish_path = post_finish_path
	return mgr, nil
}

func search_java_home(root string) string {
	java_execute := "java.exe"
	if "windows" != runtime.GOOS {
		java_execute = "java"
	}

	jp := filepath.Join(root, "runtime_env/jdk/bin", java_execute)
	if fileExists(jp) {
		return jp
	}

	jp = filepath.Join(root, "runtime_env/jre/bin", java_execute)
	if fileExists(jp) {
		return jp
	}

	ss, _ := filepath.Glob(filepath.Join(root, "**", "java.exe"))
	if nil != ss && 0 != len(ss) {
		return ss[0]
	}

	jh := os.Getenv("JAVA_HOME")
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

	fmt.Println("===================== output begin =====================")
	defer func() {
		fmt.Println("=====================  output end  =====================")
	}()
	return cmd.Run()
}

func loadConfigs(root, file, execute string) (*Manager, error) {
	var arguments map[string]interface{}
	//"autostart_"
	if 0 != len(file) {
		var e error
		arguments, e = loadProperties(root, file)
		if nil != e {
			return nil, e
		}
	} else {
		fmt.Println("[warn] the default config file is not found.")
	}

	if nil == arguments {
		arguments = loadDefault(root, file)
	} else {
		arguments["root_dir"] = root
		arguments["config_file"] = file
		arguments["os"] = runtime.GOOS
		arguments["arch"] = runtime.GOARCH
	}

	if "" == execute {
		arguments["execute"] = "daemontools"
	} else {
		arguments["execute"] = execute
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
			supervisors, e = loadConfig(nm, arguments, supervisors)
			if nil != e {
				return nil, errors.New("load '" + nm + "' failed, " + e.Error())
			} else {
				fmt.Println("load '" + nm + "' is ok.")
			}
		}
	}

	logPath := filepath.Clean(abs(filepath.Join(root, "logs")))
	logs := []string{stringWithDefault(arguments, "logPath", logPath),
		filepath.Clean(abs(filepath.Join(root, "..", "logs"))),
		logPath}

	for _, s := range logs {
		if dirExists(s) {
			logPath = s
			break
		}
	}

	if !dirExists(logPath) {
		os.Mkdir(logPath, 0660)
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
		out, e := newRotateFile(filepath.Clean(abs(filepath.Join(logPath, s.name()+".log"))), maxBytes, maxNum)
		if nil != e {
			return nil, errors.New("open log failed for '" + s.name() + "', " + e.Error())
		}
		s.setOutput(out)
	}

	return &Manager{settings_file: file, settings: arguments, supervisors: supervisors}, nil
}

func loadConfig(file string, args map[string]interface{}, supervisors []supervisor) ([]supervisor, error) {
	t, e := template.ParseFiles(file)
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
	e = json.Unmarshal(buffer.Bytes(), &v)
	//var attributes map[string]interface{}
	//e = json.Unmarshal(buffer.Bytes(), &attributes)
	if nil != e {
		fmt.Println(buffer.String())
		return nil, errors.New("ummarshal file failed, " + e.Error())
	}
	switch value := v.(type) {
	case map[string]interface{}:
		arguments := []map[string]interface{}{value, args}
		return loadSupervisor(file, arguments, supervisors)
	case []interface{}:
		for idx, o := range value {
			attributes, ok := o.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("[%v] is not a map or array.", idx)
			}

			arguments := []map[string]interface{}{attributes, args}
			supervisors, e = loadSupervisor(file, arguments, supervisors)
			if nil != e {
				return nil, e
			}
		}
		return supervisors, nil
	}
	return nil, fmt.Errorf("it is not a map or array - %T", v)
}

func loadSupervisor(file string, arguments []map[string]interface{}, supervisors []supervisor) ([]supervisor, error) {
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

	success_flag := stringWithArguments(arguments, "success_flag", "")
	if 0 == len(success_flag) {
		retries1 := intWithDefault(arguments[0], "retries", 0)
		if retries1 > 0 {
			fmt.Println("[warn] retries will ignore while success_flag is missing in '" + name + "' at '" + file + "'.")
		}
	}

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
			supervisorBase: supervisorBase{proc_name: name,
				mode:        stringWithArguments(arguments, "mode", ""),
				retries:     retries,
				killTimeout: killTimeout,
				start_cmd:   start,
				stop_cmd:    stop}})

	} else {
		supervisors = append(supervisors, &supervisor_default{success_flag: success_flag,
			supervisorBase: supervisorBase{proc_name: name,
				mode:        stringWithArguments(arguments, "mode", ""),
				retries:     retries,
				killTimeout: killTimeout,
				start_cmd:   start,
				stop_cmd:    stop}})
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
			proc = *java_path
		}
	}

	return &command{proc: proc, arguments: arguments, environments: environments, directory: directory}, nil
}

func loadJavaArguments(arguments []string, args []map[string]interface{}) ([]string, error) {

	var results []string
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
			results = append(results, "-agentlib:jdwp=transport=dt_socket,server=y,suspend=y,address="+debug)
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
	return map[string]interface{}{"root_dir": root,
		"config_file": file,
		"java":        *java_path,
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH}
}

func loadProperties(root, file string) (map[string]interface{}, error) {
	t, e := template.ParseFiles(file)
	if nil != e {
		return nil, errors.New("read config '" + file + "' failed, " + e.Error())
	}
	args := loadDefault(root, file)

	var buffer bytes.Buffer
	e = t.Execute(&buffer, args)
	if nil != e {
		return nil, errors.New("generate config '" + file + "' failed, " + e.Error())
	}

	var arguments map[string]interface{}
	e = json.Unmarshal(buffer.Bytes(), &arguments)
	if nil != e {
		return nil, errors.New("ummarshal config '" + file + "' failed, " + e.Error())
	}

	if s, ok := arguments["java"]; ok {
		*java_path = fmt.Sprint(s)
	} else {
		arguments["java"] = *java_path
	}

	return arguments, nil
}
