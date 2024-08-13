package database

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
	"time"
)

// TODOs:
// * DRY up load / write pattern when mutating DB
// * Client-specific refresh tokens

const (
	fileMode = 0666
)

var (
	ErrDoesNotExist  = errors.New("does not exist")
	ErrAlreadyExists = errors.New("already exists")
)

type Chirp struct {
	ID       int    `json:"id"`
	AuthorID int    `json:"author_id"`
	Body     string `json:"body"`
}

type User struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

type AuthUser struct {
	User
	Password []byte `json:"password"`
}

type RefreshToken struct {
	Token      string
	UserID     int
	Expiration time.Time
}

type DBRepresentation struct {
	LastChirpID    int                     `json:"last_chirp_id"`
	LastUserID     int                     `json:"last_user_id"`
	Chirps         map[int]Chirp           `json:"chirps"`
	Users          map[int]AuthUser        `json:"users"`
	UserEmailIndex map[string]int          `json:"user_email_idx"`
	RefreshTokens  map[string]RefreshToken `json:"refresh_tokens"`
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
	_, ok := db.data.UserEmailIndex[email]
	if ok {
		return User{}, ErrAlreadyExists
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
	db.mux.RLock()
	defer db.mux.RUnlock()
	_, ok := db.data.UserEmailIndex[email]
	return ok
}

func (db *DB) GetAuthUserByEmail(email string) (AuthUser, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	userID, ok := db.data.UserEmailIndex[email]
	if !ok {
		return AuthUser{}, ErrDoesNotExist
	}
	user, ok := db.data.Users[userID]
	if !ok {
		return AuthUser{}, ErrDoesNotExist
	}
	return user, nil
}

func (db *DB) UpdateUser(userID int, email string, pwHash []byte) (User, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	err := db.loadDB()
	if err != nil {
		return User{}, err
	}
	user, ok := db.data.Users[userID]
	if !ok {
		return User{}, ErrDoesNotExist
	}
	delete(db.data.UserEmailIndex, user.Email)
	user.Email = email
	user.Password = pwHash
	db.data.Users[userID] = user
	db.data.UserEmailIndex[user.Email] = user.ID
	err = db.writeDB()
	if err != nil {
		return User{}, err
	}
	return user.User, nil
}

func (db *DB) CreateRefreshToken(token string, userID int, expiresInSeconds int) (RefreshToken, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	err := db.loadDB()
	if err != nil {
		return RefreshToken{}, err
	}
	_, ok := db.data.RefreshTokens[token]
	if ok {
		return RefreshToken{}, ErrAlreadyExists
	}
	rt := RefreshToken{
		Token:      token,
		UserID:     userID,
		Expiration: time.Now().UTC().Add(time.Duration(expiresInSeconds) * time.Second),
	}
	db.data.RefreshTokens[rt.Token] = rt
	err = db.writeDB()
	if err != nil {
		return RefreshToken{}, err
	}
	return rt, nil
}

func (db *DB) GetRefreshToken(token string) (RefreshToken, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	rt, ok := db.data.RefreshTokens[token]
	if !ok {
		return RefreshToken{}, ErrDoesNotExist
	}
	return rt, nil
}

func (db *DB) DeleteRefreshToken(token string) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	err := db.loadDB()
	if err != nil {
		return err
	}
	delete(db.data.RefreshTokens, token)
	err = db.writeDB()
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) CreateChirp(body string, authorID int) (Chirp, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	err := db.loadDB()
	if err != nil {
		return Chirp{}, err
	}
	newChirp := Chirp{
		ID:       db.data.LastChirpID + 1,
		AuthorID: authorID,
		Body:     body,
	}
	db.data.Chirps[newChirp.ID] = newChirp
	db.data.LastChirpID = newChirp.ID
	err = db.writeDB()
	if err != nil {
		return Chirp{}, err
	}
	return newChirp, nil
}

func (db *DB) GetChirp(chirpID int) (Chirp, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	c, ok := db.data.Chirps[chirpID]
	if !ok {
		return Chirp{}, ErrDoesNotExist
	}
	return c, nil
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
			RefreshTokens:  make(map[string]RefreshToken),
		},
		mux: &sync.RWMutex{},
	}
	err := newDB.ensureDB()
	if err != nil {
		return nil, err
	}
	return newDB, nil
}
