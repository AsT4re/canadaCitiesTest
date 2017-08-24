package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}


/*
 *  Tests
 */

// Test found id
func TestFoundId(t *testing.T) {
	req, _ := http.NewRequest("GET", "/id/42", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusOK, response.Code)

	expected := CityTempl {
		42,
		"Amherstburg",
		8921,
		[]float64{-83.108128, 42.100072},
	}

	var result CityTempl
	checkJsonBody(t, req, response, &expected, &result)
}

// Test not found id
func TestNotFoundId(t *testing.T) {
	id := "4234534"
	req, _ := http.NewRequest("GET", fmt.Sprintf("/id/%s", id), nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusNotFound, response.Code)

	expected := ErrorRep{fmt.Sprintf(ErrNotFound, id)}

	var result ErrorRep
	checkJsonBody(t, req, response, &expected, &result)
}

// Test for invalid dist qs parameter
func TestInvalidDistParam(t *testing.T) {
	dist := "ideij"
	req, _ := http.NewRequest("GET", fmt.Sprintf("/id/42?dist=%s", dist), nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusBadRequest, response.Code)

	expected := ErrorRep{fmt.Sprintf(ErrInvalidUIntQsParam, dist, "dist")}

	var result ErrorRep
	checkJsonBody(t, req, response, &expected, &result)
}

// Test for invalid query string parameters in URL
func TestUnknownQsParam(t *testing.T) {
	req, _ := http.NewRequest("GET", "/id/42?dedo=67", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusBadRequest, response.Code)

	expected := ErrorRep{ErrUnknownQsParam}

	var result ErrorRep
	checkJsonBody(t, req, response, &expected, &result)
}

// Test for invalid query string parameters in URL with valid 'dist'
func TestUnknownQsParamMultiple(t *testing.T) {
	dist := "10"
	req, _ := http.NewRequest("GET", fmt.Sprintf("/id/42?dist=%s&fko=67", dist), nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusBadRequest, response.Code)

	expected := ErrorRep{ErrUnknownQsParam}

	var result ErrorRep
	checkJsonBody(t, req, response, &expected, &result)
}


/*
 *  Helpers
 */

func executeRequest(req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s, _ := NewServer()
	s.router.ServeHTTP(rr, req)
	return rr
}

func checkJsonBody(t *testing.T, req *http.Request, response *httptest.ResponseRecorder, expected, result interface{}) {
	if err := json.Unmarshal(response.Body.Bytes(), result); err != nil {
		t.Errorf("Invalid json object as response:\n%s\n", string(response.Body.Bytes()))
		return
	}

	const mismatchError = `
For API %s %s
Error: Json differs from expected.
Result: %s
Expected: %s
  `

	if reflect.DeepEqual(result, expected) == false {
		var out bytes.Buffer
		json.Indent(&out, response.Body.Bytes(), "", "  ")
		outExp, _ := json.MarshalIndent(expected, "", "  ")
	 	t.Errorf(mismatchError, req.Method, req.URL.Path, string(out.Bytes()), string(outExp))
	}
}

func checkResponseCode(t *testing.T, expected, result int) {
	if expected != result {
		t.Errorf("Expected response code %d. Got %d\n", expected, result)
	}
}
