package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/thomasem/chirpy/internal/database"
	"github.com/thomasem/chirpy/internal/password"
)

const (
	contentTypeHeader    = "Content-Type"
	textPlainContentType = "text/plain; charset=utf-8"
	htmlContentType      = "text/html"
	jsonContentType      = "application/json"
)

type User struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

type userRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	userRequest
	ExpiresInSeconds int `json:"expires_in_seconds"`
}

type loginResponse struct {
	User
	Token string `json:"token"`
}

type Chirp struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

type chirpRequest struct {
	Body string `json:"body"`
}

type chirpyService struct {
	fileserverHits int
	metricsMux     *sync.RWMutex
	db             *database.DB
	jwtSecret      string
}

func cleanBody(body string) string {
	badWords := []string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Split(body, " ")
	for i := range words {
		for _, bw := range badWords {
			if strings.ToLower(words[i]) == bw {
				words[i] = "****"
				break
			}
		}
	}
	return strings.Join(words, " ")
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	er := errorResponse{Error: msg}
	respondWithJSON(w, code, er)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling JSON response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentTypeHeader, jsonContentType)
	w.WriteHeader(code)
	w.Write(data)
}

func (cs *chirpyService) generateJWTString(userID int, expiresInSeconds int) (string, error) {
	secondsInDay := 60 * 60 * 24 * time.Second
	expireDuration := time.Duration(expiresInSeconds) * time.Second
	if expireDuration == 0 || expireDuration > secondsInDay {
		expireDuration = secondsInDay
	}
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(expireDuration)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy",
		Subject:   fmt.Sprintf("%v", userID),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	})
	s, err := token.SignedString([]byte(cs.jwtSecret))
	return s, err
}

func (cs *chirpyService) readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentTypeHeader, textPlainContentType)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cs *chirpyService) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.metricsMux.Lock()
		cs.fileserverHits++
		cs.metricsMux.Unlock()
		next.ServeHTTP(w, r)
	})
}

func (cs *chirpyService) metricsHandler(w http.ResponseWriter, r *http.Request) {
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
	cs.metricsMux.RLock()
	defer cs.metricsMux.RUnlock()
	fmt.Fprintf(w, template, cs.fileserverHits)
}

func (cs *chirpyService) resetHandler(w http.ResponseWriter, r *http.Request) {
	cs.metricsMux.Lock()
	cs.fileserverHits = 0
	cs.metricsMux.Unlock()
	w.Header().Set(contentTypeHeader, textPlainContentType)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Metrics reset!"))
}

func (cs *chirpyService) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	cr := chirpRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&cr)
	if err != nil {
		log.Printf("error decoding chirp request: %s", err)
		respondWithError(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	if cr.Body == "" {
		respondWithError(w, http.StatusBadRequest, "Chirp body missing")
		return
	}
	if len(cr.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	newChirp, err := cs.db.CreateChirp(cleanBody(cr.Body))
	if err != nil {
		log.Printf("error creating chirp in database: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create new chirp")
		return
	}
	respondWithJSON(w, http.StatusCreated, Chirp{
		ID:   newChirp.ID,
		Body: newChirp.Body,
	})
}

func (cs *chirpyService) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	chirps := cs.db.GetChirps()
	response := make([]Chirp, 0, len(chirps))
	for _, chirp := range chirps {
		response = append(response, Chirp{
			ID:   chirp.ID,
			Body: chirp.Body,
		})
	}
	respondWithJSON(w, http.StatusOK, response)
}

func (cs *chirpyService) getChirpHandler(w http.ResponseWriter, r *http.Request) {
	chirpIDStr := r.PathValue("chirpID")
	chirpID, err := strconv.Atoi(chirpIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Unexpected path value: %s", chirpIDStr))
		return
	}
	chirp, ok := cs.db.GetChirp(chirpID)
	if !ok {
		respondWithError(w, http.StatusNotFound, "Chirp not found")
		return
	}
	respondWithJSON(w, http.StatusOK, Chirp{
		ID:   chirp.ID,
		Body: chirp.Body,
	})
}

func (cs *chirpyService) createUserHandler(w http.ResponseWriter, r *http.Request) {
	ur := userRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&ur)
	if err != nil {
		log.Printf("error decoding user request: %s", err)
		respondWithError(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	if ur.Email == "" || ur.Password == "" {
		respondWithError(w, http.StatusBadRequest, "User missing required fields")
		return
	}
	exists := cs.db.UserExists(ur.Email)
	if exists {
		respondWithError(w, http.StatusConflict, "User already exists")
		return
	}
	pwHash, err := password.StringToHash(ur.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "User password invalid")
		return
	}
	user, err := cs.db.CreateUser(ur.Email, pwHash)
	if err != nil {
		log.Printf("error creating new user in database: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create new user")
		return
	}
	respondWithJSON(w, http.StatusCreated, User{
		ID:    user.ID,
		Email: user.Email,
	})
}

func (cs *chirpyService) loginHandler(w http.ResponseWriter, r *http.Request) {
	lr := loginRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&lr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid login request")
		return
	}
	au, ok := cs.db.GetAuthUserByEmail(lr.Email)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	if !password.Matches(lr.Password, au.Password) {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	token, err := cs.generateJWTString(au.ID, lr.ExpiresInSeconds)
	if err != nil {
		log.Printf("error generating JWT: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Unable to generate token for user")
		return
	}
	respondWithJSON(w, http.StatusOK, loginResponse{
		User: User{
			ID:    au.ID,
			Email: au.Email,
		},
		Token: token,
	})
}

func (cs *chirpyService) getUsersHandler(w http.ResponseWriter, r *http.Request) {
	users := cs.db.GetUsers()
	response := make([]User, 0, len(users))
	for _, user := range users {
		response = append(response, User{
			ID:    user.ID,
			Email: user.Email,
		})
	}
	respondWithJSON(w, http.StatusOK, response)
}

func NewChirpyService(db *database.DB, jwtSecret string) *chirpyService {
	return &chirpyService{
		db:             db,
		metricsMux:     &sync.RWMutex{},
		fileserverHits: 0,
		jwtSecret:      jwtSecret,
	}
}
