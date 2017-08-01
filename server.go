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

// type Properties struct {
// 	name       string     `json:"name"`
// 	place_key  string     `json:"place_key"`
// 	capital    string     `json:"capital"`
// 	population float64    `json:"population"`
// 	pclass     string     `json:"pclass"`
// 	cartodb_id int64      `json:"cartodb_id"`
// 	created_at string     `json:"created_at"`
// 	updated_at string     `json:"updated_at"`
// }

// type Feature struct {
// //	geometry   string     `json:"geometry"`
// 	properties Properties `json:"properties"`
// }

// type Features struct {
// 	features   []Feature  `json:"features"`
// }

func Import(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal("Error reading request body")
	}
	r.Body.Close()

	fmt.Printf("Beginning of body: ", string(body[:100]))

	var feats map[string]interface{}
	err = json.Unmarshal(body, &feats)
	//x.Checkf(err, "While unmarshalling Json file")
	if err != nil {
	 	log.Fatal("Error at Unmarshal")
	}

	fmt.Printf("Nb features: %v", len(feats))
}
