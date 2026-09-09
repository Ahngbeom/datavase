package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Ahngbeom/datavase/internal/config"
)

// dsFields is what the form holds, as typed. Turning it into a DataSource
// is separate from the widgets so the rules can be tested without a screen.
type dsFields struct {
	name, host, port, user, database                   string
	tls, tlsCA                                         string
	tunnelHost, tunnelPort, tunnelUser, tunnelIdentity string
}

func fieldsFromDataSource(ds *config.DataSource) dsFields {
	f := dsFields{
		name: ds.Name, host: ds.Host, port: strconv.Itoa(ds.Port), user: ds.User,
		database: ds.Database, tls: string(ds.TLS), tlsCA: ds.TLSCA,
	}
	if ds.Tunnel != nil {
		f.tunnelHost, f.tunnelUser, f.tunnelIdentity = ds.Tunnel.Host, ds.Tunnel.User, ds.Tunnel.Identity
		f.tunnelPort = strconv.Itoa(ds.Tunnel.Port)
	}
	return f
}

// toDataSource parses the typed values. An empty port is the default; an
// empty tunnel host with nothing else typed in the tunnel section means no
// tunnel, and with something typed means a mistake worth naming.
func (f dsFields) toDataSource() (config.DataSource, error) {
	port, err := parsePort(f.port, config.DefaultPort, "port")
	if err != nil {
		return config.DataSource{}, err
	}

	ds := config.DataSource{
		Name: strings.TrimSpace(f.name), Host: strings.TrimSpace(f.host), Port: port,
		User: strings.TrimSpace(f.user), Database: strings.TrimSpace(f.database),
		TLS: config.TLSMode(f.tls), TLSCA: strings.TrimSpace(f.tlsCA),
	}

	tunnelTyped := strings.TrimSpace(f.tunnelPort) != "" || strings.TrimSpace(f.tunnelUser) != "" ||
		strings.TrimSpace(f.tunnelIdentity) != ""
	if host := strings.TrimSpace(f.tunnelHost); host != "" {
		tport, err := parsePort(f.tunnelPort, config.DefaultTunnelPort, "tunnel port")
		if err != nil {
			return config.DataSource{}, err
		}
		ds.Tunnel = &config.Tunnel{Host: host, Port: tport,
			User: strings.TrimSpace(f.tunnelUser), Identity: strings.TrimSpace(f.tunnelIdentity)}
	} else if tunnelTyped {
		return config.DataSource{}, errors.New("tunnel host is required when the tunnel section is filled in")
	}
	return ds, nil
}

func parsePort(s string, def int, what string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("%s must be a number between 1 and 65535", what)
	}
	return n, nil
}

// validateDataSource is what Save checks before the file is touched: the
// fields the connection cannot do without, and a name that is not already
// someone else's. editing is the name the entry had, so renaming to itself
// is not a duplicate.
func validateDataSource(cfg *config.Config, editing string, ds config.DataSource) error {
	switch {
	case ds.Name == "":
		return errors.New("name is required")
	case ds.Host == "":
		return errors.New("host is required")
	case ds.User == "":
		return errors.New("user is required")
	}
	for i := range cfg.DataSources {
		other := cfg.DataSources[i].Name
		if other == ds.Name && other != editing {
			return fmt.Errorf("a datasource named %q already exists", ds.Name)
		}
	}
	if ds.Tunnel != nil && ds.Tunnel.User == "" {
		return errors.New("tunnel user is required")
	}
	return nil
}
