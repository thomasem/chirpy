package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/thomasem/chirpy/internal/database"
)

const (
	addr         = "localhost:8080"
	dbPath       = "database.json"
	jwtSecretEnv = "JWT_SECRET"
)

type errorResponse struct {
	Error string `json:"error"`
}

func configureRoutes(mux *http.ServeMux, cs *chirpyService) {
	// Admin
	mux.Handle("GET /admin/metrics", http.HandlerFunc(cs.metricsHandler))

	// API
	mux.Handle("GET /api/healthz", http.HandlerFunc(cs.readyHandler))
	mux.Handle("GET /api/reset", http.HandlerFunc(cs.resetHandler))
	mux.Handle("POST /api/chirps", http.HandlerFunc(cs.createChirpHandler))
	mux.Handle("GET /api/chirps", http.HandlerFunc(cs.getChirpsHandler))
	mux.Handle("GET /api/chirps/{chirpID}", http.HandlerFunc(cs.getChirpHandler))
	mux.Handle("POST /api/users", http.HandlerFunc(cs.createUserHandler))
	mux.Handle("GET /api/users", http.HandlerFunc(cs.getUsersHandler))
	mux.Handle("POST /api/login", http.HandlerFunc(cs.loginHandler))

	// App
	appHandler := http.FileServer(http.Dir('.'))
	mux.Handle("/app/*", http.StripPrefix("/app", cs.middlewareMetricsInc(appHandler)))
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("could not load environment variables: %s", err)
	}
	jwtSecret := os.Getenv(jwtSecretEnv)
	if jwtSecret == "" {
		log.Fatalf("'%s' not set in environment variables", jwtSecretEnv)
	}

	dbg := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()

	mux := http.NewServeMux()
	srv := http.Server{
		Handler: mux,
		Addr:    addr,
	}

	db, err := database.NewDB(dbPath, *dbg)
	if err != nil {
		log.Fatalf("error getting DB connection: %s", err)
	}
	cs := NewChirpyService(db, jwtSecret)

	configureRoutes(mux, cs)

	// TODO: add graceful handling of signals and shutdown later
	log.Printf("Serving on %s", srv.Addr)
	err = srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
