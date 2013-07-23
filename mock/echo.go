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
		ln, _ := reader.ReadLine()
		fmt.Println(ln)
		if os.Args[1] == ln {
			break
		}
	}
}
