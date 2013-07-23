package main

import (
	"bufio"
	"fmt"
	"net"
	"net/textproto"
	"os"
)

func main() {
	ln, err := net.Listen("tcp", os.Args[1])
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("listen ok")

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		reader := textproto.NewReader(bufio.NewReader(conn))
		for {
			line, e := reader.ReadLine()
			if nil != e {
				fmt.Println(e)
				break
			}

			fmt.Println(line)

			if "exit" == line {
				goto end
			}

			_, e = conn.Write([]byte(line + "\r\n"))
			if nil != e {
				fmt.Println(e)
				break
			}
		}
	}
end:
	fmt.Println("exit listen")
}
