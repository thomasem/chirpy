package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/thomasem/chirpy/internal/database"
)

type chirpRequest struct {
	Body string `json:"body"`
}

type chirpService struct {
	db *database.DB
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

func (cs *chirpService) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	cr := chirpRequest{}
	err := decoder.Decode(&cr)
	if err != nil {
		log.Printf("Error decoding chirp request: %s", err)
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
		respondWithError(w, http.StatusInternalServerError, "Failed to create new chirp, try again later")
	}
	respondWithJSON(w, http.StatusCreated, newChirp)
}

func (cs *chirpService) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	chirps := cs.db.GetChirps()
	respondWithJSON(w, http.StatusOK, chirps)
}

func (cs *chirpService) getChirpHandler(w http.ResponseWriter, r *http.Request) {
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

func NewChirpService(db *database.DB) *chirpService {
	return &chirpService{
		db: db,
	}
}
