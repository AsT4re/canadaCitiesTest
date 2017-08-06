package main

import (
//	"html"
	"log"
	"net/http"
	"github.com/gorilla/mux"
	"encoding/json"
	"io/ioutil"
	"fmt"
	"time"
)

type Server struct {
	db *DGClient
	router *mux.Router
}

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
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
			Handler(route.HandlerFunc)
	}

	return s, nil
}

// Start server
func (s *Server) Start() {
	defer s.db.Close()
	log.Fatal(http.ListenAndServe(":8443", s.router))
}

// Handlers

type ErrorRep struct {
	Error   string `json:"error"`
}

// Status Reply
type StatusRep struct {
	Message string `json:"message"`
}

// Status
func StatusHandler(s *Server) http.HandlerFunc {
	return func (w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		repJson := StatusRep{Message: "Cancities Server running on port 8443"}
		if err := json.NewEncoder(w).Encode(repJson); err != nil {
			panic(err)
		}
	}
}

type GeoJSON struct {
	Features   []struct {
		Type          string     `json:"type"`
		Geometry struct {
			Type        string     `json:"type"`
			Coordinates []float64  `json:"coordinates"`
		}                        `json:"geometry"`
		Properties struct {
			Name        string     `json:"name"`
			Place_key   string     `json:"place_key"`
			Capital     string     `json:"capital"`
			Population  int64      `json:"population"`
			Pclass      string     `json:"pclass"`
			Cartodb_id  int64      `json:"cartodb_id"`
			Created_at  time.Time  `json:"created_at"`
			Updated_at  time.Time  `json:"updated_at"`
		}                        `json:"properties"`
	}                          `json:"features"`
}

func ImportHandler(s *Server) http.HandlerFunc {
	return func (w http.ResponseWriter, r *http.Request) {
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

		s.db.AddGeoJSON(&feats)

		fmt.Printf("Nb features: %v", len(feats.Features))
	}
}

// Status Reply
type FindRep struct {
	CartodbId   int64     `json:"cartodb_id"`
	Name        string    `json:"name"`
	Population  int64     `json:"population"`
	Coordinates []float64 `json:"coordinates"`
}

func FindHandler(s *Server) http.HandlerFunc {
	return func (w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		vars := mux.Vars(r)
		var rep interface{}
		cityId := vars["id"]
		if city, err := s.db.GetCity(cityId); err != nil {
			fmt.Printf("(FindHandler) error get city: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			rep = ErrorRep{Error: "Internal Error"}
		} else if city.Root == nil {
			w.WriteHeader(http.StatusNotFound)
			rep = ErrorRep{Error: fmt.Sprintf("City with id %v not found", cityId)}
		} else {
			if geo, err := DecodeGeoDatas(city.Root.Geo); err != nil {
				fmt.Printf("(FindHandler) error decoding geo datas: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				rep = ErrorRep{Error: "Internal Error"}
			} else {
				rep = FindRep{
					CartodbId: city.Root.Cartodb_id,
					Name: city.Root.Name,
					Population: city.Root.Population,
					Coordinates: geo.FlatCoords(),
				}
				w.WriteHeader(http.StatusOK)
			}
		}
		if err := json.NewEncoder(w).Encode(rep); err != nil {
			panic(err)
		}
	}
}

