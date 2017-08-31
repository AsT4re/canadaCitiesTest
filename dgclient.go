package main

import (
	"github.com/pkg/errors"
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
	if dgCl.conn, err = grpc.Dial("127.0.0.1:9080", grpc.WithInsecure()); err != nil {
		return nil, errors.Wrap(err, "error dialing grpc connection")
	}

	if dgCl.clientDir, err = ioutil.TempDir("", "client_"); err != nil {
		return nil, errors.Wrap(err, "error creating temporary directory")
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
        cartodb_id: int @index(int) .
        geo: geo @index(geo) .
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

	if _, err := dgCl.dg.Run(context.Background(), &req); err != nil {
		return errors.Wrap(err, "error running request for schema")
	}

	return nil
}

// Close to cleanly exit at the end of the program
func (dgc *DGClient) Close() {
	if dgc.conn != nil {
		if err := dgc.conn.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "%+v\n", errors.Wrap(err, "Warning: closing connection failed:"))
		}
	}

	if dgc.clientDir != "" {
		if err := os.RemoveAll(dgc.clientDir); err != nil {
			fmt.Fprintf(os.Stderr, "%+v\n", errors.Wrap(err, "Warning: removing temp dir failed:"))
		}
	}
}

// Method for importing GeoJson
func (dgCl *DGClient) AddGeoJSON(feats *ImportReq) error {
	for _, feat := range feats.Features {
		mnode, err := dgCl.dg.NodeBlank("")
		if err != nil {
			return errors.Wrap(err, "error creating blank node")
		}

		if err = addEdge(dgCl, &mnode, "cartodb_id", feat.Properties.Cartodb_id); err != nil {
			return errors.Wrap(err, "error adding edge")
		}
		if err = addEdge(dgCl, &mnode, "name", feat.Properties.Name); err != nil {
			return errors.Wrap(err, "error adding edge")
		}
		if err = addEdge(dgCl, &mnode, "place_key", feat.Properties.Place_key); err != nil {
			return errors.Wrap(err, "error adding edge")
		}
		if err = addEdge(dgCl, &mnode, "capital", feat.Properties.Capital); err != nil {
			return errors.Wrap(err, "error adding edge")
		}
		if err = addEdge(dgCl, &mnode, "population", feat.Properties.Population); err != nil {
			return errors.Wrap(err, "error adding edge")
		}
		if err = addEdge(dgCl, &mnode, "pclass", feat.Properties.Pclass); err != nil {
			return errors.Wrap(err, "error adding edge")
		}
		if err = addEdge(dgCl, &mnode, "created_at", feat.Properties.Created_at); err != nil {
			return errors.Wrap(err, "error adding edge")
		}
		if err = addEdge(dgCl, &mnode, "updated_at", feat.Properties.Updated_at); err != nil {
			return errors.Wrap(err, "error adding edge")
		}

		buf := bytes.Buffer{}
		if err := json.NewEncoder(&buf).Encode(feat.Geometry); err != nil {
			fmt.Printf("(DGClient) Error while encoding to Json feat.Geometry: %v", err)
			return errors.Wrap(err, "error serializing json")
		}
		geoStr := buf.String()

		if err = addEdge(dgCl, &mnode, "geo", geoStr); err != nil {
			return errors.Wrap(err, "error adding edge")
		}
	}

	dgCl.dg.BatchFlush()

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
		return nil, errors.Wrap(err, "error unmarshalling for decode geo datas")
	} else {
		return vc, nil
	}
}



/*
 *  Private functions
 */

func addEdge(dgCl *DGClient, mnode *client.Node, name string, value interface{}) error {
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
		return errors.Wrapf(err, "error while setting value for %v edge with value %v", name, value)
	}

	if err = dgCl.dg.BatchSet(e); err != nil {
		return errors.Wrapf(err, "error when setting batch for %v edge with value %v", name, value)
	}

	return nil
}

func SendRequest(dgCl *DGClient, reqStr *string, reqMap *map[string]string, rep interface{}) error {
	req := client.Req{}
	req.SetQueryWithVariables(*reqStr, *reqMap)

	resp, err := dgCl.dg.Run(context.Background(), &req)
	if err != nil {
		return errors.Wrap(err, "error when executing request")
	}

	if len(resp.N[0].Children) == 0 {
		return nil
	}

	if err = client.Unmarshal(resp.N, rep); err != nil {
		return errors.Wrap(err, "error when unmarshaling dgraph reply")
	}

	return nil
}
