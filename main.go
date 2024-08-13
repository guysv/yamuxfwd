package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"

	"github.com/hashicorp/yamux"
)

type yamuxReadWriter struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (y yamuxReadWriter) Close() error {
	return nil
}

func (y yamuxReadWriter) Read(p []byte) (int, error) {
	return y.r.Read(p)
}

func (y yamuxReadWriter) Write(p []byte) (int, error) {
	return y.w.Write(p)
}

func listen(port int, yamuxConn io.ReadWriteCloser) {
	session, err := yamux.Client(yamuxConn, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating yamux client:", err)
		os.Exit(1)
	}
	defer session.Close()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error listening:", err)
		os.Exit(1)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error accepting connection:", err)
			continue
		}
		go func() {
			defer conn.Close()

			stream, err := session.Open()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error opening stream:", err)
				return
			}
			defer stream.Close()

			go io.Copy(stream, conn)
			io.Copy(conn, stream)
		}()
	}
}

func connect(addr string, yamuxConn io.ReadWriteCloser) {
	session, err := yamux.Server(yamuxConn, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating yamux client:", err)
		os.Exit(1)
	}
	defer session.Close()

	for {
		stream, err := session.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error accepting stream:", err)
			continue
		}
		go func() {
			defer stream.Close()

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error connecting to host:", err)
				return
			}
			defer conn.Close()

			go io.Copy(stream, conn)
			io.Copy(conn, stream)
		}()
	}
}

func main() {
	portPtr := flag.Int("l", 0, "listen port")
	addrPtr := flag.String("c", "", "destination address")
	reversePtr := flag.Bool("R", false, "reverse mode")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [OPTIONS] <command>\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	listenMode := *portPtr != 0
	args := flag.Args()

	if !listenMode {
		if *addrPtr == "" {
			fmt.Fprintln(os.Stderr, "Either -l [port] or -c [address] must be provided")
			os.Exit(1)
		}
	}

	var yamuxConn io.ReadWriteCloser
	if listenMode == *reversePtr {
		yamuxConn = yamuxReadWriter{r: os.Stdin, w: os.Stdout}
	} else {
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "A command must be provided")
			os.Exit(1)
		}

		cmd := exec.Command(args[0], args[1:]...)
		stdinPipe, err := cmd.StdinPipe()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error creating stdin pipe:", err)
			os.Exit(1)
		}
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error creating stdout pipe:", err)
			os.Exit(1)
		}

		cmd.Stderr = os.Stderr

		err = cmd.Start()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error starting subcommand:", err)
			os.Exit(1)
		}

		yamuxConn = yamuxReadWriter{r: stdoutPipe, w: stdinPipe}
	}

	if !listenMode {
		connect(*addrPtr, yamuxConn)
	} else {
		listen(*portPtr, yamuxConn)
	}
}
