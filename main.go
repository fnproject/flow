package main

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func ping(w http.ResponseWriter, r *http.Request) {
	return
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/ping", ping)
	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":8000", r))
}
