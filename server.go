package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"github.com/gorilla/mux"
)


func main() {
    router := mux.NewRouter().StrictSlash(false)
    router.HandleFunc("/", Index)
    log.Fatal(http.ListenAndServe(":8443", router))
}

func Index(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello !", html.EscapeString(r.URL.Path))
}
