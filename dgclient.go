package main

import (
	"errors"
	"fmt"
	"context"
	"io/ioutil"
	"os"
	"bytes"
	"strconv"
  "time"
	"encoding/json"
	"google.golang.org/grpc"
	"github.com/dgraph-io/dgraph/client"
	"github.com/twpayne/go-geom/encoding/wkb"
	geom "github.com/twpayne/go-geom"
)



/*
 * Public structures
 */

type cityProps struct {
	Name        string       `dgraph:"name"`
	Population  int64        `dgraph:"population"`
	Cartodb_id  int64        `dgraph:"cartodb_id"`
	Geo         []byte       `dgraph:"geo"`
}

// Reply structure from GetCity request
type cityRep struct {
 	Root        *cityProps   `dgraph:"city"`
}

// Reply structure from GetCitiesAround request
type citiesRep struct {
	Root        []*cityProps `dgraph:"cities"`
}

// Main class for handling dgraph database requests
type DGClient struct {
	conn      *grpc.ClientConn
	clientDir string
	dg        *client.Dgraph
}



/*
 * DGClient object with constructor and public methods
 */

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

	_, err := dgCl.dg.Run(context.Background(), &req)
	if err != nil {
		fmt.Printf("(DGClient) Error while running mutation schema request: %v", err)
		return err
	}

	return nil
}

// Close to cleanly exit at the end of the program
func (dgc *DGClient) Close() {
	if dgc.conn != nil {
		dgc.conn.Close()
	}

	if dgc.clientDir != "" {
		os.RemoveAll(dgc.clientDir)
	}
}

// Method for importing GeoJson
func (dgCl *DGClient) AddGeoJSON(feats *ImportReq) error {
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

// Method for getting informations about a specific city given his id
func (dgCl *DGClient) GetCity(id string) (cityRep, error){
	getCityTempl := `{
    city(func: eq(cartodb_id, $id)) {
      name
      geo
      cartodb_id
      population
    }
  }`

	reqMap := make(map[string]string)
	reqMap["$id"] = id

	var city cityRep
	err := SendRequest(dgCl, &getCityTempl, &reqMap, &city)
	return city, err
}

// Method for getting informations about cities within a bounding box given his center coordinates and distance in kilometers
func (dgCl *DGClient) GetCitiesAround(pos []float64, dist uint64) (citiesRep, error){
	minLat, minLong, maxLat, maxLong := getBoundingBox(pos[0], pos[1], float64(dist))

	bndBox := [5][2]float64{
		{minLong, minLat},
		{maxLong, minLat},
		{maxLong, maxLat},
		{minLong, maxLat},
		{minLong, minLat},
	}

	var buffer bytes.Buffer
	buffer.WriteString("[")
	for i := 0 ; i < 5; i++ {
		if i != 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString("[")
		for j := 0; j < 2; j++ {
			if j != 0 {
				buffer.WriteString(", ")
			}
			buffer.WriteString(strconv.FormatFloat(bndBox[i][j], 'f', -1, 64))
		}
		buffer.WriteString("]")
	}
	buffer.WriteString("]")

	getCitiesAroundTempl := `{
    cities(func: within(geo, $bndBox)) {
      name
      geo
      cartodb_id
      population
    }
  }`

	reqMap := make(map[string]string)
	reqMap["$bndBox"] = buffer.String()

	var cities citiesRep
	err := SendRequest(dgCl, &getCitiesAroundTempl, &reqMap, &cities)
	return cities, err
}



/*
 *  Public helpers
 */

// Helper for decoding geodatas in binary format
func DecodeGeoDatas(geo []byte) (geom.T, error) {
	if vc, err := wkb.Unmarshal(geo); err != nil {
		fmt.Printf("(DGClient) Error when calling wkb.unmarshal: %v", err)
		return nil, err
	} else {
		return vc, nil
	}
}



/*
 *  Private functions
 */

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

func SendRequest(dgCl *DGClient, reqStr *string, reqMap *map[string]string, rep interface{}) error {
	req := client.Req{}
	req.SetQueryWithVariables(*reqStr, *reqMap)

	resp, err := dgCl.dg.Run(context.Background(), &req)
	if err != nil {
		fmt.Printf("(DGClient) Error while executing the GetCity request: %v", err)
		return err
	}

	if len(resp.N[0].Children) == 0 {
		return nil
	}

	if err = client.Unmarshal(resp.N, rep); err != nil {
		fmt.Printf("(DGClient) error while unmarshal dgraph reply: %v", err)
		return err
	}

	return nil
}
