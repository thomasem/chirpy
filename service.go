package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/thomasem/chirpy/internal/database"
	"github.com/thomasem/chirpy/internal/password"
)

type chirpRequest struct {
	Body string `json:"body"`
}

type userRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type chirpyService struct {
	db *database.DB
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	er := errorResponse{Error: msg}
	respondWithJSON(w, code, er)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling JSON response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentTypeHeader, jsonContentType)
	w.WriteHeader(code)
	w.Write(dat)
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
	respondWithJSON(w, http.StatusCreated, newChirp)
}

func (cs *chirpyService) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	chirps := cs.db.GetChirps()
	respondWithJSON(w, http.StatusOK, chirps)
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
	respondWithJSON(w, http.StatusOK, chirp)
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
	respondWithJSON(w, http.StatusCreated, user)
}

func (cs *chirpyService) loginHandler(w http.ResponseWriter, r *http.Request) {
	ur := userRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&ur)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid login request")
		return
	}
	hash, ok := cs.db.GetUserPasswordHash(ur.Email)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Unable to find User")
		return
	}
	if !password.Matches(ur.Password, hash) {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	respondWithJSON(w, http.StatusOK, "OK")
}

func (cs *chirpyService) getUsersHandler(w http.ResponseWriter, r *http.Request) {
	users := cs.db.GetUsers()
	respondWithJSON(w, http.StatusOK, users)
}

func NewChirpyService(db *database.DB) *chirpyService {
	return &chirpyService{
		db: db,
	}
}
