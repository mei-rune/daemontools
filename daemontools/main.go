package main

import (
	"flag"
	"log"

	"github.com/runner-mei/daemontools"
)

func main() {
	flag.Parse()
	if nil != flag.Args() && 0 != len(flag.Args()) {
		flag.Usage()
		return
	}

	mgr, e := daemontools.New()
	if nil != e {
		log.Println(e)
		return
	}
	mgr.RunForever()
}
