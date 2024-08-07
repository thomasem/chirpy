package main

import (
	"fmt"
	"net/http"
)

const (
	contentTypeHeader    = "Content-Type"
	textPlainContentType = "text/plain; charset=utf-8"
)

type apiConfig struct {
	fileserverHits int
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This seems like it needs a mutex, but we'll skip for now
		cfg.fileserverHits++
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentTypeHeader, textPlainContentType)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hits: %v", cfg.fileserverHits)
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits = 0
	w.Header().Set(contentTypeHeader, textPlainContentType)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("metrics reset"))
}

func readyCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentTypeHeader, textPlainContentType)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func configureRoutes(mux *http.ServeMux) {
	cfg := apiConfig{} // maybe pass in later
	appHandler := http.FileServer(http.Dir('.'))
	mux.Handle("/app/*", http.StripPrefix("/app", cfg.middlewareMetricsInc(appHandler)))
	mux.Handle("GET /metrics", http.HandlerFunc(cfg.metricsHandler))
	mux.Handle("/reset", http.HandlerFunc(cfg.resetHandler))
	mux.Handle("GET /healthz", http.HandlerFunc(readyCheck))
}

func main() {
	mux := http.NewServeMux()
	srv := http.Server{
		Handler: mux,
		Addr:    "localhost:8080",
	}
	defer srv.Close()

	configureRoutes(mux)

	err := srv.ListenAndServe()
	if err != nil {
		fmt.Printf("Server failure: %s\n", err)
	}
}
