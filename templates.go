package main

import (
	"time"
)

// Import Request Template
type ImportReq struct {
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

const ErrNotFound = "City with id %v not found"
const ErrInvalidUIntQsParam = "Invalid uint query string value '%v' for parameter '%v'"
const ErrUnknownQsParam = "Unknown query string parameters"

// Error Reply Template
type ErrorRep struct {
	Error           string     `json:"error"`
}

// Status Reply Template
type StatusRep struct {
	Message         string     `json:"message"`
}

// Find Reply Template
type CityTempl struct {
	CartodbId       int64      `json:"cartodb_id"`
	Name            string     `json:"name"`
	Population      int64      `json:"population"`
	Coordinates     []float64  `json:"coordinates"`
}

type CitiesTempl struct {
	Cities          []CityTempl `json:"cities"`
}
