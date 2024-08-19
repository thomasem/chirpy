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
	polkaKeyEnv  = "POLKA_API_KEY"
)

type errorResponse struct {
	Error string `json:"error"`
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "assets/logo.png")
}

func configureRoutes(mux *http.ServeMux, cs *chirpyService) {
	// Admin
	mux.Handle("GET /admin/metrics", http.HandlerFunc(cs.metricsHandler))

	// Unauthenticated API
	mux.Handle("GET /api/healthz", http.HandlerFunc(cs.readyHandler))
	mux.Handle("GET /api/reset", http.HandlerFunc(cs.resetHandler))
	mux.Handle("GET /api/chirps", http.HandlerFunc(cs.getChirpsHandler))
	mux.Handle("GET /api/chirps/{chirpID}", http.HandlerFunc(cs.getChirpHandler))
	mux.Handle("POST /api/users", http.HandlerFunc(cs.createUserHandler))
	mux.Handle("GET /api/users", http.HandlerFunc(cs.getUsersHandler))

	// Password Authenticated API
	mux.Handle("POST /api/login", http.HandlerFunc(cs.loginHandler))

	// Refresh Token Authenticated API
	mux.Handle("POST /api/refresh", http.HandlerFunc(cs.refreshTokenHandler))
	mux.Handle("POST /api/revoke", http.HandlerFunc(cs.refreshTokenRevokeHandler))

	// JWT Authenticated API
	mux.Handle("PUT /api/users", http.HandlerFunc(cs.updateUserHandler))
	mux.Handle("POST /api/chirps", http.HandlerFunc(cs.createChirpHandler))
	mux.Handle("DELETE /api/chirps/{chirpID}", http.HandlerFunc(cs.deleteChirpHandler))

	// Polka Webhooks
	mux.Handle("POST /api/polka/webhooks", http.HandlerFunc(cs.polkaWebhookHandler))

	// App
	appHandler := http.FileServer(http.Dir('.'))
	mux.Handle("/favicon.ico", http.HandlerFunc(faviconHandler))
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

	polkaKey := os.Getenv(polkaKeyEnv)
	if polkaKey == "" {
		log.Fatalf("'%s' not set in environment variables", polkaKeyEnv)
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
	cs := NewChirpyService(db, jwtSecret, polkaKey)

	configureRoutes(mux, cs)

	// TODO: add graceful handling of signals and shutdown later
	log.Printf("Serving on %s", srv.Addr)
	err = srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
