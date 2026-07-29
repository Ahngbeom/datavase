package secret

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

// runStoreContract exercises the behaviour every Store implementation must
// share. Running it against both the in-memory fake and the real keychain
// wrapper is what makes it safe for other packages to test against the fake.
func runStoreContract(t *testing.T, newStore func(t *testing.T) Store) {
	t.Helper()

	t.Run("round trips a password", func(t *testing.T) {
		s := newStore(t)
		if err := s.Set("prod-app", "hunter2"); err != nil {
			t.Fatalf("Set() error = %v, want nil", err)
		}

		got, err := s.Get("prod-app")
		if err != nil {
			t.Fatalf("Get() error = %v, want nil", err)
		}
		if got != "hunter2" {
			t.Errorf("Get() = %q, want %q", got, "hunter2")
		}
	})

	t.Run("reports a missing account as ErrNotFound", func(t *testing.T) {
		s := newStore(t)
		_, err := s.Get("absent")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Get() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("overwrites an existing password", func(t *testing.T) {
		s := newStore(t)
		if err := s.Set("prod-app", "old"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := s.Set("prod-app", "new"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		got, err := s.Get("prod-app")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got != "new" {
			t.Errorf("Get() = %q, want %q", got, "new")
		}
	})

	t.Run("deletes a password", func(t *testing.T) {
		s := newStore(t)
		if err := s.Set("prod-app", "hunter2"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := s.Delete("prod-app"); err != nil {
			t.Fatalf("Delete() error = %v, want nil", err)
		}

		_, err := s.Get("prod-app")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Get() after Delete error = %v, want ErrNotFound", err)
		}
	})

	t.Run("reports deleting a missing account as ErrNotFound", func(t *testing.T) {
		s := newStore(t)
		if err := s.Delete("absent"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Delete() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("keeps accounts isolated", func(t *testing.T) {
		s := newStore(t)
		if err := s.Set("a", "pw-a"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := s.Set("b", "pw-b"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		got, err := s.Get("a")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got != "pw-a" {
			t.Errorf("Get(%q) = %q, want %q", "a", got, "pw-a")
		}
	})
}

func TestMemoryStore(t *testing.T) {
	runStoreContract(t, func(t *testing.T) Store {
		return NewMemory()
	})
}

func TestKeychainStore(t *testing.T) {
	runStoreContract(t, func(t *testing.T) Store {
		keyring.MockInit()
		return NewKeychain()
	})
}
