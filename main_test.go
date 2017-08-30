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

// Test bad route
func TestRouteNotFound(t *testing.T) {
	req, _ := http.NewRequest("GET", "/ilalded", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusNotFound, response.Code)
	checkContentType(t, JsonContentType, response.HeaderMap.Get("Content-Type"))

	expected := ErrorRep{fmt.Sprintf(ErrRouteNotFound, req.Method, req.URL.Path)}

	var result ErrorRep
	checkJsonBody(t, req, response.Body.Bytes(), &expected, &result)
}

// Test route get id with bad id
func TestBadId(t *testing.T) {
	req, _ := http.NewRequest("GET", "/id/rt23u", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusNotFound, response.Code)
	checkContentType(t, JsonContentType, response.HeaderMap.Get("Content-Type"))

	expected := ErrorRep{fmt.Sprintf(ErrRouteNotFound, req.Method, req.URL.Path)}

	var result ErrorRep
	checkJsonBody(t, req, response.Body.Bytes(), &expected, &result)
}

// Test found id
func TestFoundId(t *testing.T) {
	req, _ := http.NewRequest("GET", "/id/42", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusOK, response.Code)
	checkContentType(t, JsonContentType, response.HeaderMap.Get("Content-Type"))

	expected := CityTempl {
		42,
		"Amherstburg",
		8921,
		[]float64{-83.108128, 42.100072},
	}

	var result CityTempl
	checkJsonBody(t, req, response.Body.Bytes(), &expected, &result)
}

// Test not found id
func TestNotFoundId(t *testing.T) {
	id := "4234534"
	req, _ := http.NewRequest("GET", fmt.Sprintf("/id/%s", id), nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusNotFound, response.Code)
	checkContentType(t, JsonContentType, response.HeaderMap.Get("Content-Type"))

	expected := ErrorRep{fmt.Sprintf(ErrNotFoundId, id)}

	var result ErrorRep
	checkJsonBody(t, req, response.Body.Bytes(), &expected, &result)
}

// Test for invalid dist qs parameter
func TestInvalidDistParam(t *testing.T) {
	dist := "ideij"
	req, _ := http.NewRequest("GET", fmt.Sprintf("/id/42?dist=%s", dist), nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusBadRequest, response.Code)
	checkContentType(t, JsonContentType, response.HeaderMap.Get("Content-Type"))

	expected := ErrorRep{fmt.Sprintf(ErrInvalidUIntQsParam, dist, "dist")}

	var result ErrorRep
	checkJsonBody(t, req, response.Body.Bytes(), &expected, &result)
}

// Test for invalid query string parameters in URL
func TestUnknownQsParam(t *testing.T) {
	req, _ := http.NewRequest("GET", "/id/42?dedo=67", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusBadRequest, response.Code)
	checkContentType(t, JsonContentType, response.HeaderMap.Get("Content-Type"))

	expected := ErrorRep{ErrUnknownQsParam}

	var result ErrorRep
	checkJsonBody(t, req, response.Body.Bytes(), &expected, &result)
}

// Test for invalid query string parameters in URL with valid 'dist'
func TestUnknownQsParamMultiple(t *testing.T) {
	req, _ := http.NewRequest("GET", "/id/42?dist=10&fko=67", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusBadRequest, response.Code)
	checkContentType(t, JsonContentType, response.HeaderMap.Get("Content-Type"))

	expected := ErrorRep{ErrUnknownQsParam}

	var result ErrorRep
	checkJsonBody(t, req, response.Body.Bytes(), &expected, &result)
}

// Test if list of cities returned is exactly the same list (without order)
func TestCitiesAround(t *testing.T) {
	req, _ := http.NewRequest("GET", "/id/123?dist=4", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusOK, response.Code)
	checkContentType(t, JsonContentType, response.HeaderMap.Get("Content-Type"))

	expected := CitiesTempl {
		[]CityTempl {
			CityTempl {
				134,
				"Bradley",
				2500,
				[]float64{-82.411366, 42.339783},
			},
			CityTempl {
				123,
				"Jeannettes Creek",
				244,
				[]float64{-82.421253, 42.315238},
			},
			CityTempl {
				106,
				"Lighthouse",
				410,
				[]float64{-82.452364, 42.290865},
			},
		},
	}

	expMap := make(map[int64]CityTempl)
	for _, city := range expected.Cities {
		expMap[city.CartodbId] = city
	}

	body := response.Body.Bytes()
	var result CitiesTempl
	if err := json.Unmarshal(body, &result); err != nil {
		var out bytes.Buffer
		json.Indent(&out, body, "", "  ")
		t.Errorf("Invalid json object as response: %s\n", string(out.Bytes()))
		return
	}

	if len(result.Cities) != len(expected.Cities) {
		issueMismatchBodyError(t, req, expected, body)
	}

	for _, resCity := range result.Cities {
		expCity, ok := expMap[resCity.CartodbId]
		if !ok || reflect.DeepEqual(resCity, expCity) == false {
			issueMismatchBodyError(t, req, expected, body)
		}
	}
}

// Test not found id with dist
func TestNotFoundIdWithDist(t *testing.T) {
	id := "4234534"
	req, _ := http.NewRequest("GET", fmt.Sprintf("/id/%s?dist=4", id), nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusNotFound, response.Code)
	checkContentType(t, JsonContentType, response.HeaderMap.Get("Content-Type"))

	expected := ErrorRep{fmt.Sprintf(ErrNotFoundId, id)}

	var result ErrorRep
	checkJsonBody(t, req, response.Body.Bytes(), &expected, &result)
}


/*
 *  Helpers
 */

func executeRequest(req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s := new(Server)
	s.Init()
	s.Server.Handler.ServeHTTP(rr, req)
	return rr
}

// Check strict equality for Json body (even order in slices)
func checkJsonBody(t *testing.T, req *http.Request, body []byte, expected, result interface{}) {
	if err := json.Unmarshal(body, result); err != nil {
		t.Errorf("Invalid json object as response:\n%s\n", string(body))
		return
	}

	if reflect.DeepEqual(result, expected) == false {
		issueMismatchBodyError(t, req, expected, body)
	}
}

// Issue an error for this test printing an expected Json Body and the actual one
func issueMismatchBodyError(t *testing.T, req *http.Request, expectedBody interface{}, resultBody []byte) {
	const mismatchError = `
For API %s %s
Error: Json differs from expected.
Result: %s
Expected: %s
  `
	var out bytes.Buffer
	json.Indent(&out, resultBody, "", "  ")
	outExp, _ := json.MarshalIndent(expectedBody, "", "  ")
	t.Errorf(mismatchError, req.Method, req.URL.Path, string(out.Bytes()), string(outExp))
}

func checkResponseCode(t *testing.T, expected, result int) {
	if expected != result {
		t.Errorf("Expected response code %d. Got %d\n", expected, result)
	}
}

func checkContentType(t *testing.T, expected, result string) {
	if expected != result {
		t.Errorf("Content-Type mismatch.\nResult:\n %s\nExpected:\n %s\n", result, expected)
	}
}
