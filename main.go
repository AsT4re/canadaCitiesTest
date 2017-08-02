package main

import (
	"fmt"
	"flag"
)

var (
	dgraph = flag.String("h", "127.0.0.1:9080", "Dgraph gRPC server hostname + port")
)

func main() {
	s, err := NewServer()
	if err != nil {
		fmt.Printf("(Main) Error while creating Server: %v", err)
	}

	s.Start()
}
