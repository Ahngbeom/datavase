//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
)

// The account and the server, on the line that cannot be put away. The tree
// names the server too, and ⌘B takes the tree off the screen.
func TestTheTopLineNamesTheAccountAndTheServerItIsConnectedTo(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	top := strings.SplitN(h.text(), "\n", 2)[0]
	if !strings.Contains(top, "root@127.0.0.1:13306") {
		t.Errorf("the top line does not name the account and the server:\n%s", top)
	}
}
