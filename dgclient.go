package main

import (
	"errors"
	"fmt"
	"context"
	"io/ioutil"
	"os"
	"bytes"
  "time"
	"encoding/json"
	"google.golang.org/grpc"
	"github.com/dgraph-io/dgraph/client"
	"github.com/golang/protobuf/proto"
)

// Class for handling dgraph database requests
type DGClient struct {
	conn      *grpc.ClientConn
	clientDir string
	dg        *client.Dgraph
}

// DGClient constructor: initialize grpc connection and dgraph client
func NewDGClient() (*DGClient, error) {
	// Init connection to DGraph
	dgCl := new(DGClient)

	var err error
	dgCl.conn, err = grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	if err != nil {
		fmt.Printf("(DGClient) Error while initializing grpc connection: %v", err)
		return nil, err
	}

	dgCl.clientDir, err = ioutil.TempDir("", "client_")
	if err != nil {
		fmt.Printf("(DGClient) Error while creating temporary directory: %v", err)
		return nil, err
	}

	dgCl.dg = client.NewDgraphClient([]*grpc.ClientConn{dgCl.conn}, client.DefaultOptions, dgCl.clientDir)

	return dgCl, nil
}

// Initialize DB with schema
func (dgCl *DGClient) Init() error {
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

	resp, err := dgCl.dg.Run(context.Background(), &req)
	if err != nil {
		fmt.Printf("(DGClient) Error while running mutation schema request: %v", err)
		return err
	}

	fmt.Printf("Raw Response: %+v\n", proto.MarshalTextString(resp))
	return nil
}

func addEdge(name string, value interface{}, mnode *client.Node, req *client.Req) error {
	e := mnode.Edge(name)
	var err error
	switch v := value.(type) {
	case int64:
		err = e.SetValueInt(v)
	case string:
		if name == "geo" {
			err = e.SetValueGeoJson(v)
		} else {
			err = e.SetValueString(v)
		}
	case time.Time:
		err = e.SetValueDatetime(v)
	case float64:
		err = e.SetValueFloat(v)
	default:
		return errors.New("Type for value not handled yet")
	}

	if err != nil {
		fmt.Printf("(DGClient) Error while setting value for %v edge with value %v: %v", name, value, err)
		return err
	}
	err = req.Set(e)
	if err != nil {
		fmt.Printf("(DGClient) Error while setting value for %v edge with value %v: %v", name, value, err)
		return err
	}

	return nil
}

func (dgCl *DGClient) AddGeoJSON(feats *GeoJSON) error {
	for _, feat := range feats.Features {
		req := client.Req{}
		mnode, err := dgCl.dg.NodeBlank("")
		if err != nil {
			fmt.Printf("(DGClient) Error while creating blank node: %v", err)
			return err
		}

		if err = addEdge("cartodb_id", feat.Properties.Cartodb_id, &mnode, &req); err != nil {
			return err
		}
		if err = addEdge("name", feat.Properties.Name, &mnode, &req); err != nil {
			return err
		}
		if err = addEdge("place_key", feat.Properties.Place_key, &mnode, &req); err != nil {
			return err
		}
		if err = addEdge("capital", feat.Properties.Capital, &mnode, &req); err != nil {
			return err
		}
		if err = addEdge("population", feat.Properties.Population, &mnode, &req); err != nil {
			return err
		}
		if err = addEdge("pclass", feat.Properties.Pclass, &mnode, &req); err != nil {
			return err
		}
		if err = addEdge("created_at", feat.Properties.Created_at, &mnode, &req); err != nil {
			return err
		}
		if err = addEdge("updated_at", feat.Properties.Updated_at, &mnode, &req); err != nil {
			return err
		}

		buf := bytes.Buffer{}
		if err := json.NewEncoder(&buf).Encode(feat.Geometry); err != nil {
			fmt.Printf("(DGClient) Error while encoding to Json feat.Geometry: %v", err)
			return err
		}
		geoStr := buf.String()

		if err = addEdge("geo", geoStr, &mnode, &req); err != nil {
			return err
		}

		if _, err := dgCl.dg.Run(context.Background(), &req); err != nil {
			fmt.Printf("(DGClient) Error while executing the mutation request: %v", err)
			return err
		}
	}
	return nil
}

func (dgc *DGClient) Close() {
	if dgc.conn != nil {
		dgc.conn.Close()
	}

	if dgc.clientDir != "" {
		os.RemoveAll(dgc.clientDir)
	}
}
