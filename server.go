package main

import (
	"context"
	"fmt"
	"html"
	"log"
	"flag"
	"os"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"github.com/gorilla/mux"

	"google.golang.org/grpc"

	"github.com/dgraph-io/dgraph/client"
	"github.com/dgraph-io/dgraph/x"
	"github.com/golang/protobuf/proto"
)

var (
	dgraph = flag.String("h", "127.0.0.1:9080", "Dgraph gRPC server hostname + port")
)

func main() {
	// Init connection to DGraph
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	x.Checkf(err, "While trying to dial gRPC")
	defer conn.Close()

	clientDir, err := ioutil.TempDir("", "client_")
	x.Check(err)
	defer os.RemoveAll(clientDir)

	dgraphClient := client.NewDgraphClient([]*grpc.ClientConn{conn}, client.DefaultOptions, clientDir)

	// Set database schema
	req := client.Req{}
	req.SetQuery(`
    mutation {
      schema {
        cartodb_id: int @index .
        geo: geo @index .
        name: string .
        place_key: string .
        capital: string .
        population: int .
        pclass: string .
        created_at: dateTime .
        updated_at: dateTime .
      }
    }
`)

	resp, err := dgraphClient.Run(context.Background(), &req)

	fmt.Printf("Raw Response: %+v\n", proto.MarshalTextString(resp))

	x.Checkf(err, "Error while inserting new schema")

	// Router
	router := mux.NewRouter().StrictSlash(false)
	router.HandleFunc("/", Index)
	router.HandleFunc("/import", Import)
	log.Fatal(http.ListenAndServe(":8443", router))
}

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello !", html.EscapeString(r.URL.Path))
}

// Define types to read
type Feature struct {
  Type          string     `json:"type"`
	Geometry struct {
		Type        string     `json:"type"`
		Coordinates []float64  `json:"coordinates"`
	}                        `json:"geometry"`
	Properties struct {
		Name        string     `json:"name"`
		Place_key   string     `json:"place_key"`
		Capital     string     `json:"capital"`
		Population  float64    `json:"population"`
		Pclass      string     `json:"pclass"`
		Cartodb_id  int64      `json:"cartodb_id"`
		Created_at  string     `json:"created_at"`
		Updated_at  string     `json:"updated_at"`
	}                        `json:"properties"`
}

type GeoJSON struct {
	Features   []Feature    `json:"features"`
}

func Import(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal("Error reading request body")
	}
	r.Body.Close()

	//fmt.Printf("Beginning of body: ", string(body[:100]))

	feats := GeoJSON{}

	if err := json.Unmarshal(body, &feats); err != nil {
		panic(err)
	}

	for i, feat := range feats.Features {
		if i == 0 {
			fmt.Printf("Nb features: %v", feat.Geometry.Coordinates[0])
		}
	}

	fmt.Printf("Nb features: %v", len(feats.Features))
}
