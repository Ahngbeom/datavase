package keymap

import (
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

// Under a Korean locale arrows counted two cells while the screen draws one,
// so the key reference's description column went ragged.
func TestKeyLabelsAreMeasuredAsTheScreenDrawsThemUnderACJKLocale(t *testing.T) {
	was := runewidth.DefaultCondition.EastAsianWidth
	runewidth.DefaultCondition.EastAsianWidth = true
	t.Cleanup(func() { runewidth.DefaultCondition.EastAsianWidth = was })

	for _, label := range []string{"⌘←", "Ctrl+→", "⌥↑", "⇧↓"} {
		if got, drawn := labelWidth(label), uniseg.StringWidth(label); got != drawn {
			t.Errorf("labelWidth(%q) = %d, but it is drawn in %d cells", label, got, drawn)
		}
	}
}
