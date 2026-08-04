//go:build integration

package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

// The unit tests prove the wizard writes a file it can parse. This proves the
// thing the user actually cares about: that the file it wrote opens a session.
//
// Parsing and connecting are different claims. A config that loads and then
// cannot reach the server — because the TLS default the wizard probed under is
// not the one the file ends up meaning — is exactly the failure the wizard was
// built to prevent, and only a real server can tell the two apart.
func TestTheWrittenConfigOpensASession(t *testing.T) {
	want, password := testmysql.DataSource(t)

	s := &script{
		answers: []string{
			"wizard-integration",
			want.Host,
			strconv.Itoa(want.Port),
			want.User,
			want.Database,
		},
		// The env has no default, so it is answered here like anything else.
		// testmysql labels its datasource dev, and the wizard is told the same
		// thing rather than left to guess it.
		choices:  []int{envChoiceDev},
		password: password,
	}

	path := filepath.Join(t.TempDir(), "datavase", "config.yaml")
	w := &Wizard{
		Path:         path,
		Out:          &bytes.Buffer{},
		Ask:          s.ask,
		Choose:       s.choose,
		ReadPassword: s.readPassword,
		Secrets:      secret.NewMemory(),
		Probe: func(ctx context.Context, ds *config.DataSource, password string) (string, error) {
			sess, err := session.Open(ctx, ds, password)
			if err != nil {
				return "", err
			}
			defer sess.Close()
			return sess.Conn.ServerVersion(), nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), CheckTimeout)
	defer cancel()

	if err := w.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	stored, err := w.Secrets.Get(cfg.DataSources[0].Name)
	if err != nil {
		t.Fatalf("no password was stored: %v", err)
	}

	sess, err := session.Open(ctx, &cfg.DataSources[0], stored)
	if err != nil {
		t.Fatalf("the config the wizard wrote does not open a session: %v", err)
	}
	defer sess.Close()

	if version := sess.Conn.ServerVersion(); version == "" {
		t.Error("the session reports no server version")
	}
}
