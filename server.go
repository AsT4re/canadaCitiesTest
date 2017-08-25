package main

import (
	"log"
	"net/http"
	"github.com/gorilla/mux"
	"encoding/json"
	"io/ioutil"
	"fmt"
	"strconv"
)

type Server struct {
	db *DGClient
	router *mux.Router
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

// Server constructor
func NewServer() (*Server, error) {
	s := new(Server)
	var err error

	// Init s.db
	if s.db, err = NewDGClient(); err != nil {
		fmt.Printf("(Server) Error while creating DGClient: %v", err)
		return nil, err
	}
	s.db.Init()

	// Init s.router
	routes := getRoutes(s)
	s.router = mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		s.router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.Handler)
	}

	s.router.NotFoundHandler = NotFoundHandler(s)

	return s, nil
}

// Start server
func (s *Server) Start() {
	defer s.db.Close()
	log.Fatal(http.ListenAndServeTLS(":8443", "certificates/server.crt", "certificates/server.key", s.router))
}

type httpRetMsg struct {
	Code      int
	JsonTempl interface{}
}

type appHandler func(http.ResponseWriter, *http.Request) *httpRetMsg

// Executed before sending response
func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	ret := fn(w, r)
	w.WriteHeader(ret.Code)
	if ret.Code >= 500 {
		ret.JsonTempl = ErrorRep{"Internal Error"}
	}

	if (ret.JsonTempl != nil) {
		if err := json.NewEncoder(w).Encode(ret.JsonTempl); err != nil {
			panic(err)
		}
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
			return &httpRetMsg{Code: http.StatusInternalServerError}
		}
		r.Body.Close()

		feats := ImportReq{}

		if err := json.Unmarshal(body, &feats); err != nil {
			return &httpRetMsg{
				http.StatusUnprocessableEntity,
				ErrorRep{fmt.Sprintf("Bad import request body: %v", err)},
			}
		}

		if err = s.db.AddGeoJSON(&feats); err != nil {
			return &httpRetMsg{Code: http.StatusInternalServerError}
		}

		return &httpRetMsg{Code: http.StatusCreated}
	}
}

func FindHandler(s *Server) appHandler {
	return func (w http.ResponseWriter, r *http.Request) *httpRetMsg {
		vars := mux.Vars(r)
		cityId := vars["id"]
		if city, err := s.db.GetCity(cityId); err != nil {
			fmt.Printf("(FindHandler) error get city: %v", err)
			return &httpRetMsg{Code: http.StatusInternalServerError}
		} else if city.Root == nil {
			return &httpRetMsg{
				http.StatusNotFound,
				ErrorRep{fmt.Sprintf(ErrNotFoundId, cityId)},
			}
		} else if geo, err := DecodeGeoDatas(city.Root.Geo); err != nil {
				fmt.Printf("(FindHandler) error decoding geo datas: %v", err)
				return &httpRetMsg{Code: http.StatusInternalServerError}
		} else {
			r.ParseForm()
			qsLen := len(r.Form)
			if qsLen != 0 {
				v, ok := r.Form["dist"]
				if !ok || qsLen > 1 {
					return &httpRetMsg{
							http.StatusBadRequest,
							ErrorRep{ErrUnknownQsParam},
						}
				}
				if u, err := getUIntQsParam(v, "dist"); err != nil {
					return &httpRetMsg{
							http.StatusBadRequest,
							ErrorRep{err.Error()},
						}
					} else {
						if cities, err := s.db.GetCitiesAround(geo.FlatCoords(), u); err != nil {
							fmt.Printf("(FindHandler) error get cities: %v", err)
							return &httpRetMsg{Code: http.StatusInternalServerError}
						} else {
							citiesRep := make([]CityTempl, len(cities.Root))
							for i, city := range cities.Root {
								citiesRep[i].CartodbId = city.Cartodb_id
								citiesRep[i].Name = city.Name
								citiesRep[i].Population = city.Population
								if geo, err := DecodeGeoDatas(city.Geo); err != nil {
									fmt.Printf("(FindHandler) error decoding geo datas: %v", err)
									return &httpRetMsg{Code: http.StatusInternalServerError}
								} else {
									citiesRep[i].Coordinates = geo.FlatCoords()
								}
							}
							return &httpRetMsg{
								http.StatusOK,
								CitiesTempl{
									citiesRep,
								},
							}
						}
					}
			} else {
				return &httpRetMsg{
					http.StatusOK,
					CityTempl{
						CartodbId: city.Root.Cartodb_id,
						Name: city.Root.Name,
						Population: city.Root.Population,
						Coordinates: geo.FlatCoords(),
					},
				}
			}
		}
	}
}


/*
 *  Helpers
 */


// Check validation of uint64 query string parameter
func getUIntQsParam(v []string, key string) (uint64, error) {
	if len(v) != 1 {
		return 0, fmt.Errorf("Too many values for query string parameter: %v", key)
	} else {
		if u, err := strconv.ParseUint(v[0], 10, 64); err != nil {
			return 0, fmt.Errorf(ErrInvalidUIntQsParam, v[0], key)
		} else {
			return u, nil
		}
	}
}
