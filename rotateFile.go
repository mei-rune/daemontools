package daemontools

import (
	"fmt"
	"os"
)

type rotateFile struct {
	filename     string
	maxBytes     int
	currentBytes int
	maxNum       int

	file *os.File
}

func (w *rotateFile) Write(s []byte) (int, error) {
	if w.currentBytes >= w.maxBytes {
		if err := w.initRotate(); nil != err {
			return 0, fmt.Errorf("rotate file(%q): %s\n", w.filename, err)
		}
	}

	n, err := w.file.Write(s)
	if nil != err {
		return 0, err
	}
	w.currentBytes += n
	return n, nil
}

func (w *rotateFile) WriteString(s string) (int, error) {
	if w.currentBytes >= w.maxBytes {
		if err := w.initRotate(); nil != err {
			return 0, fmt.Errorf("rotate file(%q): %s\n", w.filename, err)
		}
	}

	n, err := w.file.WriteString(s)
	if nil != err {
		return 0, err
	}
	w.currentBytes += n
	return n, nil
}

func (w *rotateFile) Flush() {
	w.file.Sync()
}

func (w *rotateFile) Sync() error {
	return w.file.Sync()
}

func (w *rotateFile) Close() error {
	return w.file.Close()
}

func NewRotateFile(fname string, maxBytes, maxNum int) (*rotateFile, error) {
	w := &rotateFile{filename: fname, maxBytes: maxBytes, currentBytes: 0, maxNum: maxNum}

	if err := w.initRotate(); nil != err {
		fmt.Fprintf(os.Stderr, "rotateFile(%q): %s\n", w.filename, err)
		return nil, err
	}

	return w, nil
}

func (w *rotateFile) initRotate() error {
	if w.file != nil {
		w.file.Close()
	}

	_, err := os.Stat(w.filename)
	if nil == err { // file exists
		fname2 := w.filename + fmt.Sprintf(".%04d", w.maxNum)
		_, err = os.Stat(fname2)
		if nil == err {
			err = os.Remove(fname2)
			if err != nil {
				return err
			}
		}

		fname1 := fname2
		for num := w.maxNum - 1; num > 0; num-- {
			fname2 = fname1
			fname1 = w.filename + fmt.Sprintf(".%04d", num)

			_, err = os.Stat(fname1)
			if nil != err {
				continue
			}
			err = os.Rename(fname1, fname2)
			if err != nil {
				return err
			}
		}

		err = os.Rename(w.filename, fname1)
		if err != nil {
			return err
		}
	}

	fd, err := os.OpenFile(w.filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	w.file = fd
	w.currentBytes = 0
	return nil
}
