package main

import (
	"fmt"
	"net/http"
	"sync"
)

type apiConfig struct {
	fileserverHits int
	mux            *sync.RWMutex
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.mux.Lock()
		cfg.fileserverHits++
		cfg.mux.Unlock()
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	template := `
		<html>

		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>

		</html>
	`
	w.Header().Set(contentTypeHeader, htmlContentType)
	w.WriteHeader(http.StatusOK)
	cfg.mux.RLock()
	defer cfg.mux.RUnlock()
	fmt.Fprintf(w, template, cfg.fileserverHits)
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits = 0
	w.Header().Set(contentTypeHeader, textPlainContentType)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Metrics reset!"))
}

func NewAPIConfig() *apiConfig {
	return &apiConfig{
		mux:            &sync.RWMutex{},
		fileserverHits: 0,
	}
}
