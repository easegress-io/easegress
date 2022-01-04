package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func tcpClient() {
	strEcho := "Hello from client! \n"
	servAddr := "127.0.0.1:10080"
	if len(os.Args) > 2 {
		servAddr = os.Args[2]
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

	_ = conn.Close()
}

func udpClient() {
	p := make([]byte, 2048)
	servAddr := "127.0.0.1:10070"
	if len(os.Args) > 2 {
		servAddr = os.Args[2]
	}
	conn, err := net.Dial("udp", servAddr)
	if err != nil {
		fmt.Printf("Some error %v", err)
		return
	}
	_, _ = fmt.Fprintf(conn, "Ping from client")
	_, err = bufio.NewReader(conn).Read(p)
	if err == nil {
		fmt.Printf("%s\n", p)
	} else {
		fmt.Printf("Some error %v\n", err)
	}
	_ = conn.Close()
}

func main() {
	protocol := ""
	if len(os.Args) > 1 {
		protocol = os.Args[1]
	}
	switch protocol {
	case "tcp":
		tcpClient()
	case "udp":
		udpClient()
	default:
		fmt.Println("Please provide udp or tcp flag.")
	}
}
