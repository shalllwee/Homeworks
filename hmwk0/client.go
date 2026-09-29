package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	readder := bufio.NewReader(conn)
	response, err := readder.ReadString('\n')
	if err != nil {
		fmt.Println(err.Error)
	}

	expected := "OK\n"

	if response == expected {
		fmt.Println("Success")
		os.Exit(0)
	} else {
		fmt.Printf("Error: '%s'\n", response)
		os.Exit(1)
	}
}
