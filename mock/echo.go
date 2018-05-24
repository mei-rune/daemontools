package main

import (
	"bufio"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"time"
)

func main() {
	fmt.Println("ok")
	reader := textproto.NewReader(bufio.NewReader(os.Stdin))
	for {
		ln, e := reader.ReadLine()
		fmt.Println("read:", ln)
		if nil != e {
			if err != io.EOF {
				fmt.Println("error:", e)
				return
			}
			time.Sleep(10 * time.Microsecond)
		}
		if os.Args[1] == ln {
			break
		}
	}
}
