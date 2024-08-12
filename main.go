package main

import (
	"flag"
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

func configureRoutes(mux *http.ServeMux, cs *chirpyService) {
	// Admin
	mux.Handle("GET /admin/metrics", http.HandlerFunc(cs.metricsHandler))

	// API
	mux.Handle("GET /api/healthz", http.HandlerFunc(readyCheck))
	mux.Handle("GET /api/reset", http.HandlerFunc(cs.resetHandler))
	mux.Handle("POST /api/chirps", http.HandlerFunc(cs.createChirpHandler))
	mux.Handle("GET /api/chirps", http.HandlerFunc(cs.getChirpsHandler))
	mux.Handle("GET /api/chirps/{chirpID}", http.HandlerFunc(cs.getChirpHandler))
	mux.Handle("POST /api/users", http.HandlerFunc(cs.createUserHandler))
	mux.Handle("GET /api/users", http.HandlerFunc(cs.getUsersHandler))
	mux.Handle("POST /api/login", http.HandlerFunc(cs.loginHandler))

	// Authenticated API
	mux.Handle("PUT /api/users", http.HandlerFunc(cs.updateUser))

	// App
	appHandler := http.FileServer(http.Dir('.'))
	mux.Handle("/app/*", http.StripPrefix("/app", cs.middlewareMetricsInc(appHandler)))
}

func main() {
	mux := http.NewServeMux()
	srv := http.Server{
		Handler: mux,
		Addr:    addr,
	}

	dbg := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()

	db, err := database.NewDB(dbPath, *dbg)
	if err != nil {
		log.Fatalf("error getting DB connection: %s", err)
	}
	cs := NewChirpyService(db)

	configureRoutes(mux, cs)

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
