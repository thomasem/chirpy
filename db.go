package main

import (
	"bytes"
	"encoding/json"
	"os"
	"sync"
)

const (
	fileMode = 0666
)

type DBRepresentation struct {
	LastID int    `json:"last_id"`
	Chirps Chirps `json:"chirps"`
}

type DB struct {
	path string
	data DBRepresentation
	mux  *sync.RWMutex
}

func (db *DB) ensureDB() error {
	db.mux.Lock()
	defer db.mux.Unlock()
	err := db.loadDB()
	if err != nil && os.IsNotExist(err) {
		return db.writeDB()
	}
	return err
}

func (db *DB) loadDB() error {
	data, err := os.ReadFile(db.path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	return decoder.Decode(&db.data)
}

func (db *DB) writeDB() error {
	data, err := json.Marshal(db.data)
	if err != nil {
		return err
	}
	return os.WriteFile(db.path, data, fileMode)
}

func (db *DB) Create(body string) (Chirp, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.loadDB()
	newChirp := Chirp{
		ID:   db.data.LastID + 1,
		Body: body,
	}
	db.data.Chirps[newChirp.ID] = newChirp
	db.data.LastID = newChirp.ID
	err := db.writeDB()
	if err != nil {
		return Chirp{}, err
	}
	return newChirp, nil
}

func (db *DB) GetAll() Chirps {
	db.mux.RLock()
	defer db.mux.RUnlock()
	return db.data.Chirps
}

func NewDB(path string) (*DB, error) {
	newDB := &DB{
		path: path,
		data: DBRepresentation{
			LastID: 0,
			Chirps: make(map[int]Chirp),
		},
		mux: &sync.RWMutex{},
	}
	err := newDB.ensureDB()
	if err != nil {
		return nil, err
	}
	return newDB, nil
}
