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

	"github.com/thomasem/chirpy/internal/auth"
	"github.com/thomasem/chirpy/internal/database"
)

const (
	contentTypeHeader   = "Content-Type"
	authorizationHeader = "Authorization"

	textPlainContentType = "text/plain; charset=utf-8"
	htmlContentType      = "text/html"
	jsonContentType      = "application/json"

	jwtExpiresInSeconds = 60 * 60           // 1 hour
	rtExpiresInSeconds  = 60 * 60 * 24 * 60 // 60 days
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
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

type JWT struct {
	Token string `json:"token"`
}

type Chirp struct {
	ID       int    `json:"id"`
	AuthorID int    `json:"author_id"`
	Body     string `json:"body"`
}

type chirpRequest struct {
	Body string `json:"body"`
}

type chirpyService struct {
	fileserverHits int
	metricsMux     *sync.RWMutex
	db             *database.DB
	jwtSecret      []byte
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

func stripBearer(av string) string {
	return strings.TrimSpace(strings.TrimPrefix(av, "Bearer"))
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	if code > 499 {
		log.Printf("Returning error from API: (HTTP %v) %s", code, msg)
	}
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

func (cs *chirpyService) generateJWT(userID int, expiresInSeconds int) (string, error) {
	subject := fmt.Sprintf("%v", userID)
	return auth.NewJWT(subject, cs.jwtSecret, expiresInSeconds)
}

func (cs *chirpyService) getUserIDFromJWT(jwtString string) (int, error) {
	claims, err := auth.GetClaimsFromJWT(jwtString, cs.jwtSecret)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(claims.Subject)
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
	token := r.Header.Get(authorizationHeader)
	if token == "" {
		respondWithError(w, http.StatusUnauthorized, "Token missing from request")
		return
	}
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer"))
	userID, err := cs.getUserIDFromJWT(token)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	cr := chirpRequest{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&cr)
	if err != nil {
		log.Printf("error decoding chirp request: %s", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON request")
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

	newChirp, err := cs.db.CreateChirp(cleanBody(cr.Body), userID)
	if err != nil {
		log.Printf("error creating chirp in database: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create new chirp")
		return
	}
	respondWithJSON(w, http.StatusCreated, Chirp{
		ID:       newChirp.ID,
		AuthorID: newChirp.AuthorID,
		Body:     newChirp.Body,
	})
}

func (cs *chirpyService) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	chirps := cs.db.GetChirps()
	response := make([]Chirp, 0, len(chirps))
	for _, chirp := range chirps {
		response = append(response, Chirp{
			ID:       chirp.ID,
			AuthorID: chirp.AuthorID,
			Body:     chirp.Body,
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
	chirp, err := cs.db.GetChirp(chirpID)
	if err == database.ErrDoesNotExist {
		respondWithError(w, http.StatusNotFound, "Chirp not found")
		return
	}
	if err != nil {
		log.Printf("Unable to get chirp from database: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Error retrieving chirp")
		return
	}
	respondWithJSON(w, http.StatusOK, Chirp{
		ID:       chirp.ID,
		AuthorID: chirp.AuthorID,
		Body:     chirp.Body,
	})
}

func (cs *chirpyService) createUserHandler(w http.ResponseWriter, r *http.Request) {
	ur := userRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&ur)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON request")
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
	pwHash, err := auth.PasswordStringToHash(ur.Password)
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
	au, err := cs.db.GetAuthUserByEmail(lr.Email)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get user")
		return
	}
	if !auth.PasswordMatches(lr.Password, au.Password) {
		respondWithError(w, http.StatusUnauthorized, "Incorrect password")
		return
	}
	jwt, err := cs.generateJWT(au.ID, jwtExpiresInSeconds)
	if err != nil {
		log.Printf("error generating JWT: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Unable to generate token for user")
		return
	}
	refreshToken, err := auth.NewRefreshToken()
	if err != nil {
		log.Printf("error generating new refresh token: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Unable to generate refresh token for user")
		return
	}
	rt, err := cs.db.CreateRefreshToken(refreshToken, au.ID, rtExpiresInSeconds)
	if err != nil {
		log.Printf("error creating refresh token in database: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Unable to save refresh token")
		return
	}
	respondWithJSON(w, http.StatusOK, loginResponse{
		User: User{
			ID:    au.ID,
			Email: au.Email,
		},
		Token:        jwt,
		RefreshToken: rt.Token,
	})
}

func (cs *chirpyService) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get(authorizationHeader)
	if token == "" {
		respondWithError(w, http.StatusUnauthorized, "Token missing from request")
		return
	}
	token = stripBearer(token)
	rt, err := cs.db.GetRefreshToken(token)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}
	if rt.Expiration.Before(time.Now().UTC()) {
		respondWithError(w, http.StatusUnauthorized, "Expired token")
		return
	}
	jwt, err := cs.generateJWT(rt.UserID, jwtExpiresInSeconds)
	if err != nil {
		log.Printf("error generating JWT: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Unable to generate token for user")
		return
	}
	respondWithJSON(w, http.StatusOK, JWT{
		Token: jwt,
	})
}

func (cs *chirpyService) refreshTokenRevokeHandler(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get(authorizationHeader)
	if token == "" {
		respondWithError(w, http.StatusUnauthorized, "Token missing from request")
		return
	}
	token = stripBearer(token)
	err := cs.db.DeleteRefreshToken(token)
	if err != nil {
		log.Printf("error revoking refresh token: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to revoke token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func (cs *chirpyService) updateUserHandler(w http.ResponseWriter, r *http.Request) {
	ur := userRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&ur)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON request")
		return
	}
	token := r.Header.Get(authorizationHeader)
	if token == "" {
		respondWithError(w, http.StatusUnauthorized, "Token missing from request")
		return
	}
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer"))
	userID, err := cs.getUserIDFromJWT(token)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	pwHash, err := auth.PasswordStringToHash(ur.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "User password invalid")
		return
	}
	updated, err := cs.db.UpdateUser(userID, ur.Email, pwHash)
	if err != nil {
		log.Printf("error updating user in database: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}
	respondWithJSON(w, http.StatusOK, User{
		ID:    updated.ID,
		Email: updated.Email,
	})
}

func NewChirpyService(db *database.DB, jwtSecret string) *chirpyService {
	return &chirpyService{
		db:             db,
		metricsMux:     &sync.RWMutex{},
		fileserverHits: 0,
		jwtSecret:      []byte(jwtSecret),
	}
}
