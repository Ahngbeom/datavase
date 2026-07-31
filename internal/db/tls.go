package db

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/go-sql-driver/mysql"
)

// The driver's reserved TLS names. Three of the five modes are one of these,
// which is why only the other two are registered.
const (
	tlsOff         = "false"
	tlsPreferred   = "preferred"
	tlsEncryptOnly = "skip-verify"
	tlsFullVerify  = "true"
)

// tlsSetting returns the value for mysql.Config.TLSConfig, registering a
// configuration first when the mode needs one.
//
// Registration is global to the driver and keyed by name, so the name carries
// the datasource: two datasources verifying against different authorities
// must not overwrite one another's roots.
func tlsSetting(ds *config.DataSource, mode config.TLSMode) (string, error) {
	switch mode {
	case config.TLSDisabled:
		return tlsOff, nil
	case config.TLSPreferred:
		return tlsPreferred, nil
	case config.TLSRequired:
		return tlsEncryptOnly, nil
	case config.TLSVerifyIdentity:
		// Verifying against the system store, name included, is precisely
		// what the driver's "true" already does.
		if ds.TLSCA == "" {
			return tlsFullVerify, nil
		}
	case config.TLSVerifyCA:
		// No shorthand exists for "chain but not name", so this is always a
		// registered configuration.
	default:
		return "", fmt.Errorf("unknown tls mode %q", mode)
	}

	// The system store is the default set of roots, and naming a file
	// replaces it rather than adding to it: an instance behind a private
	// authority should not also be satisfied by a public one.
	roots, err := rootsFor(ds.TLSCA)
	if err != nil {
		return "", err
	}

	cfg := &tls.Config{
		RootCAs:    roots,
		ServerName: ds.Host,
		MinVersion: tls.VersionTLS12,
	}
	if mode == config.TLSVerifyCA {
		verifyChainIgnoringHostname(cfg, roots)
	}

	name := "datavase-" + ds.Name
	if err := mysql.RegisterTLSConfig(name, cfg); err != nil {
		return "", fmt.Errorf("registering TLS configuration for %q: %w", ds.Name, err)
	}
	return name, nil
}

// rootsFor loads the certificate authorities to verify against, or nil for
// the system store.
//
// A named file that cannot be read is an error rather than a fallback.
// Falling back would verify the server against a different set of authorities
// than the operator asked for, and report success for doing so.
func rootsFor(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil
	}

	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading tls_ca: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("tls_ca %q holds no certificate", path)
	}
	return pool, nil
}

// verifyChainIgnoringHostname turns cfg into MySQL's VERIFY_CA: the
// certificate must chain to a trusted root, but the name on it need not match
// the address dialled.
//
// Go has no switch for this, so the chain is verified by hand with the
// library's own check disabled. The distinction earns its keep here: a
// database is routinely reached by an address its certificate was never
// issued for — the local end of an SSH tunnel is 127.0.0.1, a failover alias
// is not the instance's name — while the authority still says who answered.
func verifyChainIgnoringHostname(cfg *tls.Config, roots *x509.CertPool) {
	cfg.InsecureSkipVerify = true
	cfg.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return errors.New("the server presented no certificate")
		}

		certs := make([]*x509.Certificate, 0, len(rawCerts))
		for _, raw := range rawCerts {
			cert, err := x509.ParseCertificate(raw)
			if err != nil {
				return fmt.Errorf("parsing the server certificate: %w", err)
			}
			certs = append(certs, cert)
		}

		opts := x509.VerifyOptions{Roots: roots, Intermediates: x509.NewCertPool()}
		for _, cert := range certs[1:] {
			opts.Intermediates.AddCert(cert)
		}
		if _, err := certs[0].Verify(opts); err != nil {
			return fmt.Errorf("the server certificate does not chain to a trusted authority: %w", err)
		}
		return nil
	}
}
