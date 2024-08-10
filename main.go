package main

import (
	"log"
	"net/http"

	"github.com/thomasem/chirpy/internal/database"
)

const (
	addr   = "localhost:8080"
	dbPath = "database.json"
)

type errorResponse struct {
	Error string `json:"error"`
}

func readyCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentTypeHeader, textPlainContentType)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func configureRoutes(mux *http.ServeMux, cs *chirpyService, cfg *apiConfig) {
	// Admin
	mux.Handle("GET /admin/metrics", http.HandlerFunc(cfg.metricsHandler))

	// API
	mux.Handle("GET /api/healthz", http.HandlerFunc(readyCheck))
	mux.Handle("GET /api/reset", http.HandlerFunc(cfg.resetHandler))
	mux.Handle("POST /api/chirps", http.HandlerFunc(cs.createChirpHandler))
	mux.Handle("GET /api/chirps", http.HandlerFunc(cs.getChirpsHandler))
	mux.Handle("GET /api/chirps/{chirpID}", http.HandlerFunc(cs.getChirpHandler))
	mux.Handle("POST /api/users", http.HandlerFunc(cs.createUserHandler))
	mux.Handle("GET /api/users", http.HandlerFunc(cs.getUsersHandler))

	// App
	appHandler := http.FileServer(http.Dir('.'))
	mux.Handle("/app/*", http.StripPrefix("/app", cfg.middlewareMetricsInc(appHandler)))
}

func main() {
	mux := http.NewServeMux()
	srv := http.Server{
		Handler: mux,
		Addr:    addr,
	}

	db, err := database.NewDB(dbPath)
	if err != nil {
		log.Fatalf("error getting DB connection: %s", err)
	}
	cs := NewChirpyService(db)

	configureRoutes(mux, cs, NewAPIConfig())

	done := make(chan struct{})
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
		done <- struct{}{}
	}()
	log.Printf("Serving on %s", srv.Addr)
	<-done
}
