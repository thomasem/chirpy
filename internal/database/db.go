package database

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"sync"
)

const (
	fileMode = 0666
)

type Chirp struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

type User struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

type DBRepresentation struct {
	LastChirpID int           `json:"last_chirp_id"`
	LastUserID  int           `json:"last_user_id"`
	Chirps      map[int]Chirp `json:"chirps"`
	Users       map[int]User  `json:"users"`
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

func (db *DB) CreateUser(email string) (User, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	err := db.loadDB()
	if err != nil {
		return User{}, err
	}
	newUser := User{
		ID:    db.data.LastUserID + 1,
		Email: email,
	}
	db.data.Users[newUser.ID] = newUser
	db.data.LastUserID = newUser.ID
	err = db.writeDB()
	if err != nil {
		return User{}, err
	}
	return newUser, nil
}

func (db *DB) GetUsers() []User {
	db.mux.RLock()
	defer db.mux.RUnlock()
	users := make([]User, 0, len(db.data.Users))
	for _, user := range db.data.Users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })
	return users
}

func (db *DB) CreateChirp(body string) (Chirp, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	err := db.loadDB()
	if err != nil {
		return Chirp{}, err
	}
	newChirp := Chirp{
		ID:   db.data.LastChirpID + 1,
		Body: body,
	}
	db.data.Chirps[newChirp.ID] = newChirp
	db.data.LastChirpID = newChirp.ID
	err = db.writeDB()
	if err != nil {
		return Chirp{}, err
	}
	return newChirp, nil
}

func (db *DB) GetChirp(chirpID int) (Chirp, bool) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	c, ok := db.data.Chirps[chirpID]
	return c, ok
}

func (db *DB) GetChirps() []Chirp {
	db.mux.RLock()
	defer db.mux.RUnlock()
	chirps := make([]Chirp, 0, len(db.data.Chirps))
	for _, chirp := range db.data.Chirps {
		chirps = append(chirps, chirp)
	}
	sort.Slice(chirps, func(i, j int) bool { return chirps[i].ID < chirps[j].ID })
	return chirps
}

func NewDB(path string) (*DB, error) {
	newDB := &DB{
		path: path,
		data: DBRepresentation{
			LastChirpID: 0,
			LastUserID:  0,
			Chirps:      make(map[int]Chirp),
			Users:       make(map[int]User),
		},
		mux: &sync.RWMutex{},
	}
	err := newDB.ensureDB()
	if err != nil {
		return nil, err
	}
	return newDB, nil
}
