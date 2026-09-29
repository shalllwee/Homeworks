package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err.Error())
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := conn.Write([]byte("OK\n"))
	if err != nil {
		fmt.Println(err.Error())
	}
}
