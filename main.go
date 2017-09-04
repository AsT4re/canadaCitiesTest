package main

import (
	"context"
	"fmt"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"astare/canadaCitiesTest/server"
)

var (
	port = flag.String("port", "8443", "Server port")
	nbConns = flag.Uint("dg-conns-pool", 10, "Number of connections to DGraph")
	dgraph = flag.String("dg-host-and-port", "127.0.0.1:9080", "Dgraph database hostname and port")
	deadline = flag.Uint("deadline", 30, "Deadline for server to gracefully shutdown (in seconds)")
	cert = flag.String("tls-crt", "certificates/server.crt", "Server TLS certificate")
	key = flag.String("tls-key", "certificates/server.key", "Server TLS private key")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// The main goroutine has to handle signals in order to supervise goroutine managing server
	cSig := make(chan os.Signal, 2)
	signal.Notify(cSig, os.Interrupt, syscall.SIGTERM)
	cErr := make(chan error)
	s := new(server.Server)

	go func() {
		if err := s.Init(*port, *dgraph, *nbConns); err != nil {
			cErr <- err
			return
		}
		if err := s.Start(*cert, *key); err != nil {
			if err == http.ErrServerClosed {
				fmt.Printf("INFO: Wait for graceful shutdown of server...\n")
			} else {
				cErr <- err
			}
		}
	}()

	defer s.Close()

	select {
	case <-cSig:
		d := time.Now().Add(time.Duration(*deadline) * time.Second)
		ctx, _ := context.WithDeadline(context.Background(), d)
		if err := s.Stop(&ctx); err != nil {
			return err
		}
	case err := <-cErr:
		return err
	}

	return nil
}
