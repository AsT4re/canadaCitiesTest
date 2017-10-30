package dgclient

import (
	"github.com/pkg/errors"
	"fmt"
	"context"
	"io/ioutil"
	"os"
	"bytes"
	"strconv"
  "time"
	"google.golang.org/grpc"
	"github.com/dgraph-io/dgraph/client"
	"github.com/twpayne/go-geom/encoding/wkb"
	geom "github.com/twpayne/go-geom"
)



/*
 * Public structures
 */

type CityProps struct {
	Name        string       `json:"name"`
	Population  int64        `json:"population"`
	Cartodb_id  int64        `json:"cartodb_id"`
	Geo         []byte       `json:"geo"`
}

// Reply structure from GetCity request
type CityRep struct {
 	Root        *CityProps   `json:"city"`
}

// Reply structure from GetCitiesAround request
type CitiesRep struct {
	Root        []*CityProps `json:"cities"`
}

// Main class for handling dgraph database requests
type DGClient struct {
	conns     []*grpc.ClientConn
	clientDir string
	dg        *client.Dgraph
}


/*
 * DGClient object with constructor and public methods
 */

// DGClient constructor: initialize grpc connection and dgraph client
func NewDGClient(host string, nbConns uint) (*DGClient, error) {
	// Init connection to DGraph
	dgCl := new(DGClient)

	var err error
	if dgCl.clientDir, err = ioutil.TempDir("", "client_"); err != nil {
		return nil, errors.Wrap(err, "error creating temporary directory")
	}

	grpcConns := make([]*grpc.ClientConn, nbConns)
	for i := 0; uint(i) < nbConns; i++ {
		if conn, err := grpc.Dial(host, grpc.WithInsecure()); err != nil {
			return nil, errors.Wrap(err, "error dialing grpc connection")
		} else {
			grpcConns[i] = conn
		}
	}

	dgCl.dg = client.NewDgraphClient(grpcConns, client.DefaultOptions, dgCl.clientDir)

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
	if len(dgc.conns) > 0 {
		connsLen := len(dgc.conns)
		for i := 0; i < connsLen; i++ {
			if err := dgc.conns[i].Close(); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: %+v\n", errors.Wrap(err, "closing connection failed:"))
			}
		}
	}

	if dgc.clientDir != "" {
		if err := os.RemoveAll(dgc.clientDir); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: %+v\n", errors.Wrap(err, "removing temp dir failed:"))
		}
	}
}

// Method for importing GeoJson
func (dgCl *DGClient) AddNewNodeToBatch(name, place_key, capital, pclass, geo string,
       	                                population, cartodb_id int64,
	                                      created_at, updated_at time.Time) error {
	mnode, err := dgCl.dg.NodeBlank("")
	if err != nil {
		return errors.Wrap(err, "error creating blank node")
	}

	if err = addEdge(dgCl, &mnode, "cartodb_id", cartodb_id); err != nil {
		return errors.Wrap(err, "error adding edge")
	}
	if err = addEdge(dgCl, &mnode, "name", name); err != nil {
		return errors.Wrap(err, "error adding edge")
	}
	if err = addEdge(dgCl, &mnode, "place_key", place_key); err != nil {
		return errors.Wrap(err, "error adding edge")
	}
	if err = addEdge(dgCl, &mnode, "capital", capital); err != nil {
		return errors.Wrap(err, "error adding edge")
	}
	if err = addEdge(dgCl, &mnode, "population", population); err != nil {
		return errors.Wrap(err, "error adding edge")
	}
	if err = addEdge(dgCl, &mnode, "pclass", pclass); err != nil {
		return errors.Wrap(err, "error adding edge")
	}
	if err = addEdge(dgCl, &mnode, "created_at", created_at); err != nil {
		return errors.Wrap(err, "error adding edge")
	}
	if err = addEdge(dgCl, &mnode, "updated_at", updated_at); err != nil {
		return errors.Wrap(err, "error adding edge")
	}
	if err = addEdge(dgCl, &mnode, "geo", geo); err != nil {
		return errors.Wrap(err, "error adding edge")
	}

	return nil
}

func (dgCl *DGClient) BatchFlush() {
	dgCl.dg.BatchFlush()
}

// Method for getting informations about a specific city given his id
func (dgCl *DGClient) GetCity(id string) (CityRep, error){
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

	var city CityRep
	err := sendRequest(dgCl, &getCityTempl, &reqMap, &city)
	return city, err
}

// Method for getting informations about cities within a bounding box given his center coordinates and distance in kilometers
func (dgCl *DGClient) GetCitiesAround(pos []float64, dist uint64) (CitiesRep, error){
	minLat, minLong, maxLat, maxLong := getBoundingBox(pos[0], pos[1], float64(dist))

	bndBox := [5][2]float64{
		{minLong, minLat},
		{maxLong, minLat},
		{maxLong, maxLat},
		{minLong, maxLat},
		{minLong, minLat},
	}

	var buffer bytes.Buffer
	buffer.WriteString("[[")
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
	buffer.WriteString("]]")

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

	var cities CitiesRep
	err := sendRequest(dgCl, &getCitiesAroundTempl, &reqMap, &cities)
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

func sendRequest(dgCl *DGClient, reqStr *string, reqMap *map[string]string, rep interface{}) error {
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
