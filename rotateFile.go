package daemontools

import (
	"fmt"
	"os"
)

type RotateError interface {
	RotateToError()
}

type rotateFile struct {
	filename     string
	maxBytes     int
	currentBytes int
	maxNum       int

	file *os.File
}

func (w *rotateFile) RotateToError() {
	if err := w.initRotate(true); nil != err {
		fmt.Printf("rotate file(%q): %s\n", w.filename, err)
	}
}

func (w *rotateFile) Write(s []byte) (int, error) {
	if w.currentBytes >= w.maxBytes {
		if err := w.initRotate(false); nil != err {
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
		if err := w.initRotate(false); nil != err {
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

	if err := w.initRotate(false); nil != err {
		fmt.Fprintf(os.Stderr, "rotateFile(%q): %s\n", w.filename, err)
		return nil, err
	}

	return w, nil
}

func (w *rotateFile) initRotate(isError bool) error {
	_, err := os.Stat(w.filename)
	if nil == err { // file exists
		filename := w.filename
		if isError {
			filename = filename + ".error"
		}
		fname2 := filename + fmt.Sprintf(".%04d", w.maxNum)
		_, err = os.Stat(fname2)
		if err == nil {
			err = os.Remove(fname2)
			if err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			}
		}

		fname1 := fname2
		for num := w.maxNum - 1; num > 0; num-- {
			fname2 = fname1
			fname1 = filename + fmt.Sprintf(".%04d", num)

			_, err = os.Stat(fname1)
			if nil != err {
				continue
			}
			err = os.Rename(fname1, fname2)
			if err != nil {
				if e := os.Remove(fname1); e != nil {
					return err
				}
			}
		}


		if w.file != nil {
			w.file.Close()
		}

		err = os.Rename(w.filename, fname1)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		}
	}


	fmt.Println("log to", w.filename)
	fd, err := os.OpenFile(w.filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	w.file = fd
	w.currentBytes = 0
	return nil
}
