package main

import (
	"flag"
	"log"
	"os"

	"github.com/runner-mei/daemontools"
)

var listenAddress = flag.String("listen", ":37070", "the address of http")

func main() {
	flag.Parse()
	if nil != flag.Args() && 0 != len(flag.Args()) {
		flag.Usage()
		return
	}

	e := daemontools.Init()
	if nil != e {
		log.Println(e)
		return
	}

	mgr, e := daemontools.New(nil)
	if nil != e {
		log.Println(e)
		return
	}

	if err := daemontools.CreatePidFile(daemontools.PidFile, daemontools.Program); err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer daemontools.RemovePidFile(daemontools.PidFile)

	mgr.RunForever(*listenAddress)
}
