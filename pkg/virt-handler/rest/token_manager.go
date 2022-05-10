package rest

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const tokenRunes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

type Token struct {
	CreationTimestamp time.Time
	Value             string
	TTL               time.Duration
	Key               string
}

func (t *Token) Validate(value string) error {
	now := time.Now()
	if now.After(t.CreationTimestamp.Add(t.TTL)) {
		return fmt.Errorf("token expired")
	}
	if value != value {
		return fmt.Errorf("token is invalid")
	}
	return nil
}

type TokenManager interface {
	Create(key string) *Token
	Validate(key string, value string) error
}

type tokenManager struct {
	store           map[string]*Token
	lock            sync.Mutex
	CleanupCallback func(token *Token)
}

func NewTokenManager() *tokenManager {
	mgr := &tokenManager{
		store: map[string]*Token{},
		lock:  sync.Mutex{},
	}
	mgr.CleanupCallback = mgr.cleanupCallback
	return mgr
}

// Create creates an in-memory random token with
// a limited TTL. After TTL will be reached the token
// gets automatically removed.
func (t *tokenManager) Create(key string) *Token {
	token := &Token{
		CreationTimestamp: time.Now(),
		Value:             randString(16),
		TTL:               1 * time.Minute,
		Key:               key,
	}
	t.set(token)
	t.CleanupCallback(token)
	return token
}

// Validate will look for a token for the specified key. If it does not
// exist, it is expired or does not matche the value, it will return an error.
func (t *tokenManager) Validate(key string, value string) error {
	if token := t.get(key); token != nil {
		if err := token.Validate(value); err != nil {
			return fmt.Errorf("%s: %v", key, err)
		}
	} else {
		return fmt.Errorf("no token for %v found", key)
	}
	return nil
}

func (t *tokenManager) set(token *Token) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.store[token.Key] = token
}

func (t *tokenManager) del(token *Token) {
	t.lock.Lock()
	defer t.lock.Unlock()
	delete(t.store, token.Key)
}

func (t *tokenManager) get(key string) *Token {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.store[key]
}

func (t *tokenManager) cleanupCallback(token *Token) {
	go func(token *Token) {
		time.Sleep(token.TTL + 1*time.Second)
		if toDel := t.get(token.Key); toDel == nil || toDel.CreationTimestamp != token.CreationTimestamp {
			return
		}
		t.del(token)
	}(token)
}

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = tokenRunes[rand.Intn(len(tokenRunes))]
	}
	return string(b)
}
