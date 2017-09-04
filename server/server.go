package server

import (
	"context"
	"net/http"
	"github.com/pkg/errors"
	"github.com/gorilla/mux"
	"encoding/json"
	"io/ioutil"
	"os"
	"fmt"
	"strconv"
	"bytes"
	"astare/canadaCitiesTest/dgclient"
)


/*
 *  Private routes definition
 */

type route struct {
	name        string
	method      string
	pattern     string
	handler     appHandler
}

type routes []route

func getRoutes(s *Server) []route {
	return routes {
		route{
			"Status",
			"GET",
			"/",
			statusHandler(s),
		},
		route{
			"Import",
			"POST",
			"/import",
			importHandler(s),
		},
		route{
			"Find",
			"GET",
			"/id/{id:[0-9]+}",
			findHandler(s),
		},
	}
}


/*
 *  Server object and methods
 */

type Server struct {
	db     *dgclient.DGClient
	server *http.Server
}

const JsonContentType = "application/json; charset=UTF-8"

// Server constructor
func (s *Server) Init(port, dgraph string, nbConns uint) error {
	// Init s.db
	var err error
	if s.db, err = dgclient.NewDGClient(dgraph, nbConns); err != nil {
		return err
	}
	s.db.Init()

	// Init router
	routes := getRoutes(s)
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		router.
			Methods(route.method).
			Path(route.pattern).
			Name(route.name).
			Handler(route.handler)
	}

	router.NotFoundHandler = notFoundHandler(s)

	var buf bytes.Buffer
	buf.WriteString(":")
	buf.WriteString(port)
	// Init http server
	s.server = &http.Server{
		Addr: buf.String(),
		Handler: router,
	}

	return nil
}

func (s *Server) Start(cert, key string) error {
	if err := s.server.ListenAndServeTLS(cert, key); err != nil {
		return errors.Wrap(err, "Fail to serve")
	}

	return nil
}

func (s *Server) Stop(ctx *context.Context) error {
	if err := s.server.Shutdown(*ctx); err != nil {
		return errors.Wrap(err, "Fail to properly shutdown the server")
	}

	return nil
}

func (s *Server) Close() {
	s.db.Close()
}


/*
 *  Private Handlers
 */

type httpRetMsg struct {
	code      int
	jsonTempl interface{}
}

type appHandler func(http.ResponseWriter, *http.Request) *httpRetMsg

// Executed before sending response
func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ret := fn(w, r)
	if ret.code == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: Return code has not been set by handler\n")
		ret.code = http.StatusInternalServerError
	}

	if (ret.jsonTempl != nil) {
		w.Header().Set("Content-Type", JsonContentType)
		w.WriteHeader(ret.code)
		if err := json.NewEncoder(w).Encode(ret.jsonTempl); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Fail to serialize json body\n")
		}
	} else {
		w.WriteHeader(ret.code)
	}
}

// Route not found
func notFoundHandler(s *Server) appHandler {
	return func (w http.ResponseWriter, r *http.Request) *httpRetMsg {
		return &httpRetMsg{
			http.StatusNotFound,
			ErrorRep{fmt.Sprintf(ErrRouteNotFound, r.Method, r.URL.Path)},
		}
	}
}

func statusHandler(s *Server) appHandler {
	return func (w http.ResponseWriter, r *http.Request) *httpRetMsg {
		return &httpRetMsg{
			http.StatusOK,
			StatusRep{"Server running on port 8443"},
		}
	}
}

func importHandler(s *Server) appHandler {
	return func (w http.ResponseWriter, r *http.Request) *httpRetMsg {

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return internalError(errors.Wrap(err, "Error reading body:"))
		}

		if err = r.Body.Close(); err != nil {
			return internalError(errors.Wrap(err, "Error closing pipe:"))
		}

		feats := ImportReq{}

		if err = json.Unmarshal(body, &feats); err != nil {
			return &httpRetMsg{
				http.StatusUnprocessableEntity,
				ErrorRep{fmt.Sprintf(ErrUnprocessableEntity, err)},
			}
		}

		for _, feat := range feats.Features {
			buf := bytes.Buffer{}
			if err := json.NewEncoder(&buf).Encode(feat.Geometry); err != nil {
				return internalError(err)
			}
			err := s.db.AddNewNodeToBatch(
				feat.Properties.Name,
				feat.Properties.Place_key,
				feat.Properties.Capital,
				feat.Properties.Pclass,
				buf.String(),
				feat.Properties.Population,
				feat.Properties.Cartodb_id,
				feat.Properties.Created_at,
				feat.Properties.Updated_at)
			if err != nil {
				return internalError(err)
			}
		}

		s.db.BatchFlush()

		return &httpRetMsg{code: http.StatusCreated}
	}
}

func findHandler(s *Server) appHandler {
	return func (w http.ResponseWriter, r *http.Request) *httpRetMsg {
		vars := mux.Vars(r)
		cityId := vars["id"]

		// Get city node
		city, err := s.db.GetCity(cityId)
		if err != nil {
			return internalError(err)
		}

		// City not found
		if city.Root == nil {
			return &httpRetMsg{
				http.StatusNotFound,
				ErrorRep{fmt.Sprintf(ErrNotFoundId, cityId)},
			}
		}

		geo, err := dgclient.DecodeGeoDatas(city.Root.Geo)
		if err != nil {
			return internalError(err)
		}

		r.ParseForm()

		cityInfos := CityTempl{
			CartodbId: city.Root.Cartodb_id,
			Name: city.Root.Name,
			Population: city.Root.Population,
			Coordinates: geo.FlatCoords(),
		}

		v, ok := r.Form["dist"]
		if ok == false {
			// Simple get of city informations
			return &httpRetMsg{
				http.StatusOK,
				cityInfos,
			}
		}

		u, err := getUIntQsParam(v, "dist")
		if err != nil {
			// Bad uint parameter
			return &httpRetMsg{
				http.StatusBadRequest,
				ErrorRep{err.Error()},
			}
		}
		if u == 0 {
			// Case where dist == 0, only the city is returned
			return &httpRetMsg{
				http.StatusOK,
				CitiesTempl{
					[]CityTempl{
						cityInfos,
					},
				},
			}
		}

		var cities dgclient.CitiesRep
		cities, err = s.db.GetCitiesAround(geo.FlatCoords(), u)
		if err != nil {
			return internalError(err)
		}

		citiesArr := make([]CityTempl, len(cities.Root))
		for i, city := range cities.Root {
			citiesArr[i].CartodbId = city.Cartodb_id
			citiesArr[i].Name = city.Name
			citiesArr[i].Population = city.Population

			if geo, err := dgclient.DecodeGeoDatas(city.Geo); err != nil {
				return internalError(err)
			} else {
				citiesArr[i].Coordinates = geo.FlatCoords()
			}
		}

		return &httpRetMsg{
			http.StatusOK,
			CitiesTempl{
				citiesArr,
			},
		}
	}
}


/*
 *  Private Helpers
 */

// Print to the console + return json message internal error
func internalError(err error) *httpRetMsg {
	fmt.Fprintf(os.Stderr, "ERROR: %+v\n", err)
	return &httpRetMsg{code: http.StatusInternalServerError}
}

// Check validation of uint64 query string parameter
func getUIntQsParam(v []string, key string) (uint64, error) {
	if len(v) != 1 {
		return 0, fmt.Errorf(ErrTooManyValues, key)
	} else {
		if u, err := strconv.ParseUint(v[0], 10, 64); err != nil {
			return 0, fmt.Errorf(ErrInvalidUIntQsParam, v[0], key)
		} else {
			return u, nil
		}
	}
}
