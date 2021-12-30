package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

// TeeWriter is an io.Writer wapper.
type TeeWriter struct {
	writers []io.Writer
}

// NewTeeWriter returns a TeeWriter.
func NewTeeWriter(writers ...io.Writer) *TeeWriter {
	return &TeeWriter{writers: writers}
}

// Write writes the data.
func (tw *TeeWriter) Write(p []byte) (n int, err error) {
	for _, w := range tw.writers {
		w.Write(p)
	}
	return len(p), nil
}

func httpServer() {
	echoHandler := func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(10 * time.Millisecond)
		body, err := io.ReadAll(req.Body)
		if err != nil {
			body = []byte(fmt.Sprintf("<read failed: %v>", err))
		}

		tw := NewTeeWriter(w, os.Stdout)

		url := req.URL.Path
		if req.URL.Query().Encode() != "" {
			url += "?" + req.URL.Query().Encode()
		}

		fmt.Fprintln(tw, "Your Request")
		fmt.Fprintln(tw, "==============")
		fmt.Fprintln(tw, "Method:", req.Method)
		fmt.Fprintln(tw, "URL   :", url)

		fmt.Fprintln(tw, "Header:")
		for k, v := range req.Header {
			fmt.Fprintf(tw, "    %s: %v\n", k, v)
		}

		fmt.Fprintln(tw, "Body  :", string(body))
	}

	http.HandleFunc("/", echoHandler)
	http.HandleFunc("/pipeline", echoHandler)

	http.ListenAndServe(":9095", nil)
	fmt.Println("listen and serve failed")
}

func tcpServer() {
	echoHandler := func(conn net.Conn) {
		reader := bufio.NewReader(conn)
		for {
			message, err := reader.ReadString('\n')
			if err != nil {
				conn.Close()
				return
			}
			fmt.Printf("Message incoming: %s", string(message))
			conn.Write([]byte("Message received.\n"))
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:9095")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go echoHandler(conn)
	}
}

func udpServer() {
	echoHandler := func(pc net.PacketConn, addr net.Addr, buf []byte) {
		// 0 - 1: ID
		// 2: QR(1): Opcode(4)
		buf[2] |= 0x80 // Set QR bit

		pc.WriteTo(buf, addr)
	}
	// listen to incoming udp packets
	pc, err := net.ListenPacket("udp", ":9095")
	if err != nil {
		log.Fatal(err)
	}
	defer pc.Close()

	for {
		buf := make([]byte, 1024)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		go echoHandler(pc, addr, buf[:n])
	}
}

func main() {
	protocol := "http"
	if len(os.Args) > 1 {
		protocol = os.Args[1]
	}
	switch protocol {
	case "tcp":
		tcpServer()
	case "udp":
		udpServer()
	default:
		httpServer()
	}
}
