// Package secret stores datasource passwords outside the configuration file.
//
// The Store interface exists so that the rest of datavase never imports a
// keychain library directly; tests use the in-memory implementation and get
// the same behaviour, guaranteed by the shared contract test.
package secret

import (
	"errors"
	"fmt"
	"sync"

	"github.com/zalando/go-keyring"
)

// Service is the keychain service name every datavase entry is filed under.
// Accounts within it are datasource names.
const Service = "datavase"

// ErrNotFound reports that no password is stored for an account.
var ErrNotFound = errors.New("no password stored")

// Store holds one password per datasource name.
type Store interface {
	Get(account string) (string, error)
	Set(account, password string) error
	Delete(account string) error
}

// Keychain is the OS keychain implementation (macOS Keychain, Secret
// Service on Linux, Credential Manager on Windows).
type Keychain struct {
	service string
}

// NewKeychain returns a Store backed by the OS keychain.
func NewKeychain() *Keychain {
	return &Keychain{service: Service}
}

func (k *Keychain) Get(account string) (string, error) {
	pw, err := keyring.Get(k.service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", fmt.Errorf("%w for datasource %q", ErrNotFound, account)
	}
	if err != nil {
		return "", fmt.Errorf("reading keychain entry for %q: %w", account, err)
	}
	return pw, nil
}

func (k *Keychain) Set(account, password string) error {
	if err := keyring.Set(k.service, account, password); err != nil {
		return fmt.Errorf("writing keychain entry for %q: %w", account, err)
	}
	return nil
}

func (k *Keychain) Delete(account string) error {
	err := keyring.Delete(k.service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("%w for datasource %q", ErrNotFound, account)
	}
	if err != nil {
		return fmt.Errorf("deleting keychain entry for %q: %w", account, err)
	}
	return nil
}

// Memory is an in-memory Store for tests.
type Memory struct {
	mu sync.Mutex
	m  map[string]string
}

// NewMemory returns an empty in-memory Store.
func NewMemory() *Memory {
	return &Memory{m: make(map[string]string)}
}

func (m *Memory) Get(account string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pw, ok := m.m[account]
	if !ok {
		return "", fmt.Errorf("%w for datasource %q", ErrNotFound, account)
	}
	return pw, nil
}

func (m *Memory) Set(account, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.m[account] = password
	return nil
}

func (m *Memory) Delete(account string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.m[account]; !ok {
		return fmt.Errorf("%w for datasource %q", ErrNotFound, account)
	}
	delete(m.m, account)
	return nil
}
