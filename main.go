package main

import (
	"fmt"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	srv := http.Server{
		Handler: mux,
		Addr:    "localhost:8080",
	}
	defer srv.Close()
	err := srv.ListenAndServe()
	if err != nil {
		fmt.Printf("Server failure: %s\n", err)
	}
}
