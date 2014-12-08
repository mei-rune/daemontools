package main

import (
	"flag"
	"log"

	"github.com/runner-mei/daemontools"
)

var listenAddress = flag.String("listen", ":37070", "the address of http")

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
	mgr.RunForever(*listenAddress)
}
