package main

import (
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
)

var Router *mux.Router

func init () {
	Router = mux.NewRouter().UseEncodedPath()
	Router.StrictSlash(true)
	Router.Handle("/metrics", Handler())
}

func StartServer() *http.Server{
	port := 9000
	listenAddr := fmt.Sprintf(":%d", port)

	r := mux.NewRouter()
	//r.HandleFunc("/{bucket}", handler)

	r.PathPrefix("/").Handler(Router)

	srv := &http.Server{
		Handler: r,
		Addr:    listenAddr,
		// Good practice: enforce timeouts for servers you create!
		//WriteTimeout: 15 * time.Second,
		//ReadTimeout:  15 * time.Second,
	}

	go log.Fatal(srv.ListenAndServe())
	log.Infof("Listening on %s", listenAddr)

	return srv
}