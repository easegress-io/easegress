package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	strEcho := "Hello from client! \n"
	servAddr := "127.0.0.1:10080"
	if len(os.Args) > 1 {
		servAddr = os.Args[1]
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", servAddr)
	if err != nil {
		fmt.Println("ResolveTCPAddr failed:", err.Error())
		os.Exit(1)
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		fmt.Println("Dial failed:", err.Error())
		os.Exit(1)
	}

	_, err = conn.Write([]byte(strEcho))
	if err != nil {
		fmt.Println("Write to server failed:", err.Error())
		os.Exit(1)
	}

	fmt.Println("write to server = ", strEcho)

	reply := make([]byte, 1024)

	_, err = conn.Read(reply)
	if err != nil {
		fmt.Println("Write to server failed:", err.Error())
		os.Exit(1)
	}

	fmt.Println("reply from server=", string(reply))

	conn.Close()
}
