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
)

var (
	port = flag.String("port", "8443", "Server port")
	dgraph = flag.String("dgraph", "127.0.0.1:9080", "Dgraph hostname + port")
	deadline = flag.Uint("deadline", 30, "Deadline for server to gracefully shutdown")
	cert = flag.String("tls-crt", "certificates/server.crt", "Server TLS certificate")
	key = flag.String("tls-key", "certificates/server.key", "Server TLS private key")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// The main goroutine has to handle signals in order to supervise goroutine managing server
	cSig := make(chan os.Signal, 2)
	signal.Notify(cSig, os.Interrupt, syscall.SIGTERM)
	cErr := make(chan error)
	s := new(Server)

	go func() {
		if err := s.Init(*port, *dgraph); err != nil {
			cErr <- err
			return
		}
		if err := s.Start(*cert, *key); err != nil {
			if err == http.ErrServerClosed {
				fmt.Printf("%v\n", err)
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
