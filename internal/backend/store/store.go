package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"os"
	"sync"

	"github.com/jcarvalho/ftm-platform/internal/files"
)

type Account struct {
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Salt         string `json:"-"`
	Role         string `json:"role"`
}

type Store struct {
	path string
	mu   sync.Mutex
	data Data
}

type Data struct {
	Accounts map[string]Account
}

func Open(path string) (*Store, error) {
	store := &Store{
		path: path,
		data: Data{Accounts: map[string]Account{}},
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) EnsureAdmin(username, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.Accounts[username]; ok {
		return nil
	}
	account, err := NewAccount(username, password, "admin")
	if err != nil {
		return err
	}
	s.data.Accounts[username] = account
	return s.saveLocked()
}

func (s *Store) Authenticate(username, password string) (Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.data.Accounts[username]
	if !ok {
		return Account{}, false
	}
	return account, account.PasswordHash == hashPassword(password, account.Salt)
}

func (s *Store) SaveAccount(username, password, role string) (Account, error) {
	if role == "" {
		role = "viewer"
	}
	if role != "admin" && role != "viewer" {
		return Account{}, errors.New("role must be admin or viewer")
	}
	account, err := NewAccount(username, password, role)
	if err != nil {
		return Account{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Accounts[username] = account
	return account, s.saveLocked()
}

func (s *Store) Accounts() []Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	accounts := make([]Account, 0, len(s.data.Accounts))
	for _, account := range s.data.Accounts {
		accounts = append(accounts, account)
	}
	return accounts
}

func NewAccount(username, password, role string) (Account, error) {
	if username == "" || password == "" {
		return Account{}, errors.New("username and password are required")
	}
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return Account{}, err
	}
	salt := hex.EncodeToString(saltBytes)
	return Account{
		Username:     username,
		PasswordHash: hashPassword(password, salt),
		Salt:         salt,
		Role:         role,
	}, nil
}

func hashPassword(password, salt string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return hex.EncodeToString(sum[:])
}

func (s *Store) load() error {
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	if err := gob.NewDecoder(file).Decode(&s.data); err != nil {
		return err
	}
	if s.data.Accounts == nil {
		s.data.Accounts = map[string]Account{}
	}
	return nil
}

func (s *Store) saveLocked() error {
	if err := files.EnsureParent(s.path); err != nil {
		return err
	}
	file, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer file.Close()
	return gob.NewEncoder(file).Encode(s.data)
}
