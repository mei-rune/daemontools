package daemontools

import (
	"flag"
	"testing"
)

var test_file = flag.String("test.pid_file", "./daemontools.pid", "")

func TestPidfileExist(t *testing.T) {
	t.Log(*test_file)
	if e := createPidFile(*test_file); nil != e {
		t.Error(e)
	}
}
