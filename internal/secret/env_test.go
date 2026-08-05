package secret

import (
	"errors"
	"testing"
)

// refusingStore stands in for a keychain that is not there at all: a Linux box
// with no Secret Service answers every call with an error, and never with
// ErrNotFound.
type refusingStore struct{ err error }

func (r refusingStore) Get(string) (string, error) { return "", r.err }
func (r refusingStore) Set(string, string) error   { return r.err }
func (r refusingStore) Delete(string) error        { return r.err }

func TestEnvVarName(t *testing.T) {
	tests := []struct {
		account string
		want    string
	}{
		{"local", "DATAVASE_PASSWORD_LOCAL"},
		{"prod-app", "DATAVASE_PASSWORD_PROD_APP"},
		{"reports.eu-west-1", "DATAVASE_PASSWORD_REPORTS_EU_WEST_1"},
		{"Staging DB", "DATAVASE_PASSWORD_STAGING_DB"},
	}

	for _, tt := range tests {
		if got := EnvVarName(tt.account); got != tt.want {
			t.Errorf("EnvVarName(%q) = %q, want %q", tt.account, got, tt.want)
		}
	}
}

// A machine with no keychain is the whole reason this exists: without it the
// password cannot be supplied at all and the datasource is unusable.
func TestEnvSuppliesThePasswordWhenTheKeychainRefuses(t *testing.T) {
	t.Setenv("DATAVASE_PASSWORD_PROD_APP", "hunter2")

	s := WithEnv(refusingStore{err: errors.New("no secret service")})

	got, err := s.Get("prod-app")
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if got != "hunter2" {
		t.Errorf("Get() = %q, want %q", got, "hunter2")
	}
}

// An explicit variable has to beat the keychain, or the same command would do
// different things depending on what happens to be stored on the machine.
func TestEnvBeatsAStoredPassword(t *testing.T) {
	t.Setenv("DATAVASE_PASSWORD_PROD_APP", "from-env")

	inner := NewMemory()
	if err := inner.Set("prod-app", "from-keychain"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := WithEnv(inner).Get("prod-app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != "from-env" {
		t.Errorf("Get() = %q, want %q", got, "from-env")
	}
}

// An unset variable must not shadow the keychain, which is what every user
// with a working one relies on.
func TestUnsetEnvLeavesTheKeychainAnswering(t *testing.T) {
	inner := NewMemory()
	if err := inner.Set("prod-app", "from-keychain"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := WithEnv(inner).Get("prod-app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != "from-keychain" {
		t.Errorf("Get() = %q, want %q", got, "from-keychain")
	}
}

// An empty variable is an answer, not an absence: a database that genuinely
// has no password would otherwise be impossible to express.
func TestEmptyEnvIsAPassword(t *testing.T) {
	t.Setenv("DATAVASE_PASSWORD_LOCAL", "")

	got, err := WithEnv(refusingStore{err: errors.New("no secret service")}).Get("local")
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if got != "" {
		t.Errorf("Get() = %q, want %q", got, "")
	}
}

// The keychain's own error has to survive, so a broken keychain is reported as
// broken rather than as a datasource nobody ever set a password for.
func TestKeychainErrorSurvivesWhenNoEnvIsSet(t *testing.T) {
	refused := errors.New("no secret service")

	_, err := WithEnv(refusingStore{err: refused}).Get("prod-app")
	if !errors.Is(err, refused) {
		t.Errorf("Get() error = %v, want it to wrap %v", err, refused)
	}
}

// Wrapping must not change what the rest of datavase can assume about a Store.
func TestEnvWrappedStoreKeepsTheContract(t *testing.T) {
	runStoreContract(t, func(t *testing.T) Store {
		return WithEnv(NewMemory())
	})
}
