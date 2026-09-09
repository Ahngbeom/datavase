package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
)

func fieldsFor(name string) dsFields {
	return dsFields{name: name, host: "db.example.com", port: "3306", user: "app", tls: string(config.TLSPreferred)}
}

func TestFieldsBecomeADataSource(t *testing.T) {
	f := fieldsFor("prod")
	f.database = "shop"
	f.tunnelHost, f.tunnelPort, f.tunnelUser = "jump", "2222", "me"

	ds, err := f.toDataSource()
	if err != nil {
		t.Fatal(err)
	}
	if ds.Port != 3306 || ds.Database != "shop" || ds.Tunnel == nil || ds.Tunnel.Port != 2222 || ds.Tunnel.User != "me" {
		t.Errorf("toDataSource() = %+v", ds)
	}
}

func TestAnEmptyPortMeansTheDefault(t *testing.T) {
	f := fieldsFor("x")
	f.port, f.tunnelHost, f.tunnelPort = "", "jump", ""
	f.tunnelUser = "me"
	ds, err := f.toDataSource()
	if err != nil {
		t.Fatal(err)
	}
	if ds.Port != config.DefaultPort || ds.Tunnel.Port != config.DefaultTunnelPort {
		t.Errorf("ports = %d / %d, want the defaults", ds.Port, ds.Tunnel.Port)
	}
}

func TestANonNumericPortIsRefused(t *testing.T) {
	f := fieldsFor("x")
	f.port = "abc"
	if _, err := f.toDataSource(); err == nil || !strings.Contains(err.Error(), "port") {
		t.Errorf("toDataSource() error = %v, want one naming the port", err)
	}
}

func TestATunnelWithoutAHostIsNoTunnel(t *testing.T) {
	f := fieldsFor("x")
	f.tunnelUser = "me" // a user typed into the tunnel section, no host
	if _, err := f.toDataSource(); err == nil || !strings.Contains(err.Error(), "tunnel host") {
		t.Errorf("toDataSource() error = %v, want a refusal naming the tunnel host", err)
	}
	f.tunnelUser = ""
	ds, err := f.toDataSource()
	if err != nil || ds.Tunnel != nil {
		t.Errorf("an empty tunnel section must mean no tunnel; got %+v, %v", ds.Tunnel, err)
	}
}

func TestValidateRefusesADuplicateNameExceptTheOneBeingEdited(t *testing.T) {
	cfg := &config.Config{DataSources: []config.DataSource{{Name: "a", Host: "h", User: "u"}, {Name: "b", Host: "h", User: "u"}}}

	dup := config.DataSource{Name: "a", Host: "h", User: "u"}
	if err := validateDataSource(cfg, "", dup); err == nil {
		t.Error("adding a second \"a\" was accepted")
	}
	if err := validateDataSource(cfg, "a", dup); err != nil {
		t.Errorf("editing \"a\" under its own name was refused: %v", err)
	}
	renamed := config.DataSource{Name: "b", Host: "h", User: "u"}
	if err := validateDataSource(cfg, "a", renamed); err == nil {
		t.Error("renaming \"a\" to the existing \"b\" was accepted")
	}
}

func TestValidateRequiresNameHostAndUser(t *testing.T) {
	cfg := &config.Config{}
	for _, ds := range []config.DataSource{
		{Host: "h", User: "u"},
		{Name: "n", User: "u"},
		{Name: "n", Host: "h"},
	} {
		if err := validateDataSource(cfg, "", ds); err == nil {
			t.Errorf("validateDataSource(%+v) = nil, want an error", ds)
		}
	}
}
