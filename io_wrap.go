package daemontools

import (
	"bytes"
	"sync"
	//"fmt"
	"io"
)

func crossingMatch(s, pattern []byte) int {
	l := len(s)
	if l > len(pattern) {
		l = len(pattern)
	}

	for ; l > 0; l-- {
		if 0 == bytes.Compare(s[len(s)-l:], pattern[:l]) {
			return l
		}
	}
	return 0
}

func match(s, pattern []byte, offset int) (bool, int) {
	orign_offset := offset
	for offset > 0 {
		//fmt.Printf("s=%v\tpattern=%v\toffset=%v\r\n", string(s), string(pattern), offset)
		if (offset + len(s)) < len(pattern) {
			if 0 == bytes.Compare(s, pattern[offset:offset+len(s)]) {
				return false, offset + len(s)
			}
		} else {
			//fmt.Printf("s=%v\tpattern=%v\toffset=%v, left=%v, right=%v\r\n",
			// string(s), string(pattern), offset, string(s[:len(pattern)-offset]), string(pattern[offset:]))
			if 0 == bytes.Compare(s[:len(pattern)-offset], pattern[offset:]) {
				return true, 0
			}
		}
		offset = crossingMatch(pattern[orign_offset-offset+1:orign_offset], pattern)
	}

	if -1 != bytes.Index(s, pattern) {
		return true, 0
	}
	return false, crossingMatch(s, pattern)
}

type matchWriter struct {
	pattern []byte
	out     io.Writer
	offset  int
	matched bool
	cb      func()
}

func (self *matchWriter) Matched() bool {
	return self.matched
}

func (self *matchWriter) Write(p []byte) (n int, err error) {
	if self.matched {
		if nil != self.out {
			return self.out.Write(p)
		}
		return len(p), nil
	}

	if nil != self.out {
		n, err = self.out.Write(p)
	} else {
		n = len(p)
	}

	self.matched, self.offset = match(p[0:n], self.pattern, self.offset)
	if self.matched && nil != self.cb {
		self.cb()
	}

	//fmt.Printf("s=%v\tpattern=%v\tresult offset=%v\r\n", string(p), string(self.pattern), self.offset)
	return n, err
}

func wrap(out io.Writer, pattern []byte, cb func()) io.Writer {
	if nil == pattern || 0 == len(pattern) || nil == cb {
		return out
	}
	return &matchWriter{pattern: pattern, out: out, cb: cb}
}

type safe_writer struct {
	sync.Mutex
	out io.Writer
}

func (self *safe_writer) Write(p []byte) (n int, e error) {
	self.Lock()
	defer self.Unlock()
	return self.Write(p)
}

func safe_io(out io.Writer) io.Writer {
	return &safe_writer{out: out}
}
