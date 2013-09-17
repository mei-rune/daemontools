package daemontools

import (
	"testing"
)

func TestPidfileExist(t *testing.T) {
	if e := createPidFile("./daemontools.pid"); nil != e {
		t.Error(e)
	}
}
