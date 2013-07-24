package daemontools

import (
	"bytes"
	"encoding/json"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"text/template"
	"time"
)

var (
	is_print      = flag.Bool("print", false, "print search paths while config is not found")
	root_dir      = flag.String("root", ".", "the root directory")
	config_file   = flag.String("config", "./<program_name>.conf", "the config file path")
	listenAddress = flag.String("listen", ":9087", "the address of http")

	manager_exporter = &Exporter{}
)

func fileExist(nm string) bool {
	fs, e := os.Stat(nm)
	if nil != e {
		return false
	}
	return !fs.IsDir()
}

func dirExist(nm string) bool {
	fs, e := os.Stat(nm)
	if nil != e {
		return false
	}
	return fs.IsDir()
}

// func usage() {
// 	program := filepath.Base(os.Args[0])
// 	fmt.Fprint(os.Stderr, program, ` [options]
// Options:
// `)
// 	flag.PrintDefaults()
// }

func abs(s string) string {
	r, e := filepath.Abs(s)
	if nil != e {
		return s
	}
	return r
}

func Main() {
	flag.Parse()
	if nil != flag.Args() && 0 != len(flag.Args()) {
		flag.Usage()
		return
	}

	if "" == *root_dir {
		*root_dir = abs(filepath.Dir(os.Args[0]))
	} else {
		*root_dir = abs(*root_dir)
	}
	if !dirExist(*root_dir) {
		fmt.Println("root directory '", *root_dir, "' is not exist.")
		return
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
			filepath.Clean(abs(filepath.Join(*root_dir, "conf", "daemon.conf")))}

		found := false
		for _, nm := range files {
			if fileExist(nm) {
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
		if !fileExist(file) {
			fmt.Println("config '" + file + "' is not exists.")
			return
		}
	}

	mgr, e := loadConfigs(*root_dir, file)
	if nil != e {
		fmt.Println(e)
		return
	}

	expvar.Publish("supervisors", manager_exporter)
	manager_exporter.Var = mgr
	mgr.runForever()
}

func loadConfigs(root, file string) (*manager, error) {
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

	patterns := stringsWithDefault(arguments, "patterns", ";", nil)
	patterns = append(patterns, filepath.Clean(abs(filepath.Join(root, "autostart_*.conf"))),
		filepath.Clean(abs(filepath.Join(root, "*_autostart.conf"))),
		filepath.Clean(abs(filepath.Join(root, "*-autostart.conf"))),
		filepath.Clean(abs(filepath.Join(root, "autostart-*.conf"))))

	supervisors := make([]*supervisor, 0, 10)
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
		if dirExist(s) {
			logPath = s
			break
		}
	}

	if !dirExist(logPath) {
		os.Mkdir(logPath, 0)
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
		var e error
		s.out, e = newRotateFile(filepath.Clean(abs(filepath.Join(logPath, s.name+".log"))), maxBytes, maxNum)
		if nil != e {
			return nil, errors.New("open log failed for '" + s.name + "', " + e.Error())
		}
	}
	return &manager{supervisors: supervisors}, nil
}

func loadConfig(file string, args map[string]interface{}, supervisors []*supervisor) ([]*supervisor, error) {
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
		return loadSupervisor(arguments, supervisors)

	case []interface{}:
		for idx, o := range value {
			attributes, ok := o.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("[%v] is not a map or array.", idx)
			}

			arguments := []map[string]interface{}{attributes, args}
			supervisors, e = loadSupervisor(arguments, supervisors)
			if nil != e {
				return nil, e
			}
		}
	}
	return nil, errors.New("it is not a map or array.")
}

func loadSupervisor(arguments []map[string]interface{}, supervisors []*supervisor) ([]*supervisor, error) {

	// type supervisor struct {
	//   name        string
	//   prompt      string
	//   repected    int
	//   killTimeout time.Duration
	//   start       *command
	//   stop        *command
	// }

	name := stringWithArguments(arguments, "name", "")
	if 0 == len(name) {
		return nil, errors.New("'name' is missing.")
	}
	prompt := stringWithArguments(arguments, "prompt", "")
	repected := intWithArguments(arguments, "repected", 5)
	if repected <= 0 {
		return nil, errors.New("'repected' must is greate 0.")
	}
	killTimeout := durationWithArguments(arguments, "killTimeout", 5)
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

		start, e = loadCommand(append([]map[string]interface{}{m}, arguments[1:]...))
		if nil != e {
			return nil, e
		}
	}

	supervisors = append(supervisors, &supervisor{name: name,
		prompt:      prompt,
		repected:    repected,
		killTimeout: killTimeout,
		start:       start,
		stop:        stop})
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
	return &command{proc: proc, arguments: arguments, environments: environments, directory: directory}, nil
}

func loadDefault(root, file string) map[string]interface{} {
	return map[string]interface{}{"root_dir": root,
		"config_file": file,
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH}
}

func loadProperties(root, file string) (map[string]interface{}, error) {
	t, e := template.ParseFiles(file)
	if nil != e {
		return nil, errors.New("read config failed, " + e.Error())
	}
	args := loadDefault(root, file)

	var buffer bytes.Buffer
	e = t.Execute(&buffer, args)
	if nil != e {
		return nil, errors.New("generate config failed, " + e.Error())
	}

	var arguments map[string]interface{}
	e = json.Unmarshal(buffer.Bytes(), &arguments)
	if nil != e {
		return nil, errors.New("ummarshal config failed, " + e.Error())
	}

	return arguments, nil
}
