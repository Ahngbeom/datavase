package secret

import (
	"os"
	"strings"
)

// EnvPrefix begins the variable that supplies a datasource's password when the
// OS keychain cannot.
const EnvPrefix = "DATAVASE_PASSWORD_"

// EnvVarName returns the variable that holds account's password.
//
// Anything that cannot appear in a shell variable name becomes an underscore,
// so a datasource may keep the name that reads best in the configuration file.
// Two names that differ only in punctuation therefore collide — "prod-app" and
// "prod.app" are the same variable — which is worth knowing about but is not
// worth rejecting names over, since the config already has to hold one entry
// per datasource under a distinct name.
func EnvVarName(account string) string {
	var b strings.Builder
	b.WriteString(EnvPrefix)

	for _, r := range strings.ToUpper(account) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// envStore answers Get from the environment before asking the keychain.
type envStore struct{ inner Store }

// WithEnv returns a Store that reads a password from the environment in
// preference to inner.
//
// The keychain is the only place datavase stores a password, and on a headless
// Linux box there is no keychain to reach: go-keyring needs a D-Bus Secret
// Service, which a server does not run. Without this, the machine a terminal
// database client is most likely to be used on is the one where no password
// can be supplied at all.
//
// The environment wins rather than merely filling in, so that exporting the
// variable does the same thing everywhere. A fallback that only applied when
// the keychain happened to fail would make the same command behave differently
// on two machines, which is the harder thing to debug.
//
// Only reading is layered. A process cannot export a variable into the shell
// that started it, so Set and Delete stay with the keychain and report its
// failure honestly — see the message `dv init` prints when they do.
func WithEnv(inner Store) Store { return &envStore{inner: inner} }

func (e *envStore) Get(account string) (string, error) {
	// LookupEnv rather than Getenv: a database with no password is a real
	// configuration, and an empty variable is how it is stated.
	if pw, ok := os.LookupEnv(EnvVarName(account)); ok {
		return pw, nil
	}
	return e.inner.Get(account)
}

func (e *envStore) Set(account, password string) error { return e.inner.Set(account, password) }

func (e *envStore) Delete(account string) error { return e.inner.Delete(account) }
