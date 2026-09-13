//go:build integration

package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

// The account and the server, on the line that cannot be put away. The tree
// names the server too, and ⌘B takes the tree off the screen.
func TestTheTopLineNamesTheAccountAndTheServerItIsConnectedTo(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	// Built from the same place the harness connected, rather than written
	// out: the DATAVASE_TEST_* overrides exist so this suite can run against
	// a server on another port, which is what CI does.
	ds, _ := testmysql.DataSource(t)
	want := fmt.Sprintf("%s@%s:%d", ds.User, ds.Host, ds.Port)

	top := strings.SplitN(h.text(), "\n", 2)[0]
	if !strings.Contains(top, want) {
		t.Errorf("the top line does not name %q:\n%s", want, top)
	}
}
