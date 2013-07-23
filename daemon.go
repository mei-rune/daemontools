package daemontools

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"text/template"
)

var (
	is_print    = flag.Bool("print", false, "print search paths while config is not found")
	root_dir    = flag.String("root", ".", "the root directory")
	config_file = flag.String("config", "./<program_name>.conf", "the config file path")
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

func usage() {

	program := filepath.Base(os.Args[0])
	fmt.Fprint(os.Stderr, program, ` [options] 
Options:
`)
	flag.PrintDefaults()
}

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
			fmt.Println("config '", file, "' is not exists.")
			return
		}
	}

	mgr, e := loadConfigs(*root_dir, file)
	if nil != e {
		fmt.Println("read config file '", file, "' failed,", e)
		return
	}
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
	}

	if nil == arguments {
		arguments = loadDefault(root, file)
	}

	pattern := filepath.Clean(abs(filepath.Join(root, "autostart_*.conf")))
	matches, e := filepath.Glob(pattern)
	if nil != e {
		return nil, errors.New("search '" + pattern + "' failed, " + e.Error())
	}
	for _, nm := range matches {
		fmt.Println(nm)
	}
	return nil, errors.New("not implemented")
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
