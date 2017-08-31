package main

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
)

type Server struct {
	db     *DGClient
	Server *http.Server
}

type Route struct {
	Name        string
	Method      string
	Pattern     string
	Handler     appHandler
}

type Routes []Route

func getRoutes(s *Server) []Route {
	return Routes {
		Route{
			"Status",
			"GET",
			"/",
			StatusHandler(s),
		},
		Route{
			"Import",
			"POST",
			"/import",
			ImportHandler(s),
		},
		Route{
			"Find",
			"GET",
			"/id/{id:[0-9]+}",
			FindHandler(s),
		},
	}
}

const JsonContentType = "application/json; charset=UTF-8"

// Server constructor
func (s *Server) Init(port, dgraph string) error {
	// Init s.db
	var err error
	if s.db, err = NewDGClient(dgraph); err != nil {
		return err
	}
	s.db.Init()

	// Init router
	routes := getRoutes(s)
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.Handler)
	}

	router.NotFoundHandler = NotFoundHandler(s)

	var buf bytes.Buffer
	buf.WriteString(":")
	buf.WriteString(port)
	// Init http server
	s.Server = &http.Server{
		Addr: buf.String(),
		Handler: router,
	}

	return nil
}

// Start server
func (s *Server) Start(cert, key string) error {
	if err := s.Server.ListenAndServeTLS(cert, key); err != nil {
		return errors.Wrap(err, "Fail to serve")
	}

	return nil
}

// Stop server
func (s *Server) Stop(ctx *context.Context) error {
	if err := s.Server.Shutdown(*ctx); err != nil {
		return errors.Wrap(err, "Fail to properly shutdown the server")
	}
	return nil
}

func (s *Server) Close() {
	s.db.Close()
}

type httpRetMsg struct {
	Code      int
	JsonTempl interface{}
}

type appHandler func(http.ResponseWriter, *http.Request) *httpRetMsg

// Executed before sending response
func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ret := fn(w, r)
	if ret.Code == 0 {
		fmt.Fprintf(os.Stderr, "Error: Return code has not been set by handler\n")
		ret.Code = http.StatusInternalServerError
	}

	if (ret.JsonTempl != nil) {
		w.Header().Set("Content-Type", JsonContentType)
		w.WriteHeader(ret.Code)
		if err := json.NewEncoder(w).Encode(ret.JsonTempl); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Fail to serialize json body\n")
		}
	} else {
		w.WriteHeader(ret.Code)
	}
}


/*
 *  Handlers
 */


// Route not found
func NotFoundHandler(s *Server) appHandler {
	return func (w http.ResponseWriter, r *http.Request) *httpRetMsg {
		return &httpRetMsg{
			http.StatusNotFound,
			ErrorRep{fmt.Sprintf(ErrRouteNotFound, r.Method, r.URL.Path)},
		}
	}
}

// Status
func StatusHandler(s *Server) appHandler {
	return func (w http.ResponseWriter, r *http.Request) *httpRetMsg {
		return &httpRetMsg{
			http.StatusOK,
			StatusRep{"Cancities Server running on port 8443"},
		}
	}
}

func ImportHandler(s *Server) appHandler {
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

		if err = s.db.AddGeoJSON(&feats); err != nil {
			return internalError(err)
		}

		return &httpRetMsg{Code: http.StatusCreated}
	}
}

func FindHandler(s *Server) appHandler {
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

		geo, err := DecodeGeoDatas(city.Root.Geo)
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

		var cities citiesRep
		cities, err = s.db.GetCitiesAround(geo.FlatCoords(), u)
		if err != nil {
			return internalError(err)
		}

		citiesArr := make([]CityTempl, len(cities.Root))
		for i, city := range cities.Root {
			citiesArr[i].CartodbId = city.Cartodb_id
			citiesArr[i].Name = city.Name
			citiesArr[i].Population = city.Population

			if geo, err := DecodeGeoDatas(city.Geo); err != nil {
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
 *  Helpers
 */

// Print to the console + return json message internal error
func internalError(err error) *httpRetMsg {
	fmt.Fprintf(os.Stderr, "%+v\n", err)
	return &httpRetMsg{Code: http.StatusInternalServerError}
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
