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

type AuthUser struct {
	User
	Password []byte `json:"password"`
}

type DBRepresentation struct {
	LastChirpID    int              `json:"last_chirp_id"`
	LastUserID     int              `json:"last_user_id"`
	Chirps         map[int]Chirp    `json:"chirps"`
	Users          map[int]AuthUser `json:"users"`
	UserEmailIndex map[string]int   `json:"user_email_idx"`
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

func (db *DB) CreateUser(email string, pwHash []byte) (User, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	err := db.loadDB()
	if err != nil {
		return User{}, err
	}
	newUser := AuthUser{
		User: User{
			ID:    db.data.LastUserID + 1,
			Email: email,
		},
		Password: pwHash,
	}
	db.data.Users[newUser.ID] = newUser
	db.data.UserEmailIndex[newUser.Email] = newUser.ID
	db.data.LastUserID = newUser.ID
	err = db.writeDB()
	if err != nil {
		return User{}, err
	}
	return newUser.User, nil
}

func (db *DB) GetUsers() []User {
	db.mux.RLock()
	defer db.mux.RUnlock()
	users := make([]User, 0, len(db.data.Users))
	for _, u := range db.data.Users {
		users = append(users, u.User)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })
	return users
}

func (db *DB) UserExists(email string) bool {
	_, ok := db.data.UserEmailIndex[email]
	return ok
}

func (db *DB) GetAuthUserByEmail(email string) (AuthUser, bool) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	userID, ok := db.data.UserEmailIndex[email]
	if !ok {
		return AuthUser{}, ok
	}
	user, ok := db.data.Users[userID]
	return user, ok
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

func NewDB(path string, truncate bool) (*DB, error) {
	if truncate {
		// Might be worth having some extra guarding to ensure we don't accidentally
		// delete something we want, but for now this works OK
		os.Remove(path)
	}
	newDB := &DB{
		path: path,
		data: DBRepresentation{
			LastChirpID:    0,
			LastUserID:     0,
			Chirps:         make(map[int]Chirp),
			Users:          make(map[int]AuthUser),
			UserEmailIndex: make(map[string]int),
		},
		mux: &sync.RWMutex{},
	}
	err := newDB.ensureDB()
	if err != nil {
		return nil, err
	}
	return newDB, nil
}
