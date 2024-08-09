package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
)

type Chirp struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

type Chirps map[int]Chirp

type chirpRequest struct {
	Body string `json:"body"`
}

type chirpService struct {
	db *DB
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

	newChirp, err := cs.db.Create(cleanBody(cr.Body))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create new chirp, try again later")
	}
	respondWithJSON(w, http.StatusCreated, newChirp)
}

func (cs *chirpService) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	chirps := cs.db.GetAll()
	response := make([]Chirp, 0, len(chirps))
	for _, chirp := range chirps {
		response = append(response, chirp)
	}
	sort.Slice(response, func(i, j int) bool { return response[i].ID < response[j].ID })
	respondWithJSON(w, http.StatusOK, response)
}

func NewChirpService(db *DB) *chirpService {
	return &chirpService{
		db: db,
	}
}
