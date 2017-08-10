package main

import (
//	"html"
	"log"
	"net/http"
	"github.com/gorilla/mux"
	"encoding/json"
	"io/ioutil"
	"fmt"
)

type Server struct {
	db *DGClient
	router *mux.Router
}

type httpRetMsg struct {
	Code      int
	JsonTempl interface{}
}

type appHandler func(http.ResponseWriter, *http.Request) *httpRetMsg

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

	return s, nil
}

// Start server
func (s *Server) Start() {
	defer s.db.Close()
	log.Fatal(http.ListenAndServe(":8443", s.router))
}

// Handlers

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
				ErrorRep{fmt.Sprintf("City with id %v not found", cityId)},
			}
		} else if geo, err := DecodeGeoDatas(city.Root.Geo); err != nil {
				fmt.Printf("(FindHandler) error decoding geo datas: %v", err)
				return &httpRetMsg{Code: http.StatusInternalServerError}
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

