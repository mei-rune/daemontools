package main

import (
	"bufio"
	"fmt"
	"net/textproto"
	"os"
)

func main() {
	fmt.Println("ok")
	reader := textproto.NewReader(bufio.NewReader(os.Stdin))
	for {
		ln, e := reader.ReadLine()
		fmt.Println("read:", ln)
		if nil != e {
			fmt.Println("error:", e)
			return
		}
		if os.Args[1] == ln {
			break
		}
	}
}
