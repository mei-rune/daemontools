package main

import (
	"bufio"
	"fmt"
	"net"
	"net/textproto"
	"os"
)

func main() {
	conn, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("connect ok")

	reader := textproto.NewReader(bufio.NewReader(conn))
	for _, s := range os.Args[2:] {
		fmt.Println("send", s)
		_, e := conn.Write([]byte(s + "\r\n"))
		if nil != e {
			fmt.Println(e)
			break
		}

		if "exit" == s {
			break
		}
		ln, e := reader.ReadLine()
		if nil != e {
			fmt.Println(e)
			break
		}
		fmt.Println("recv", ln)
	}
	fmt.Println("disconnect")
}
