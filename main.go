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
	dgraph = flag.String("h", "127.0.0.1:9080", "Dgraph gRPC server hostname + port")
)

func main() {
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
		if err := s.Init(); err != nil {
			cErr <- err
			return
		}
		if err := s.Start(); err != nil {
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
		d := time.Now().Add(500000 * time.Millisecond)
		ctx, _ := context.WithDeadline(context.Background(), d)
		if err := s.Stop(&ctx); err != nil {
			return err
		}
	case err := <-cErr:
		return err
	}

	return nil
}
