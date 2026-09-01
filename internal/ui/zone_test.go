package ui

import "testing"

// A click resolves by column, so a zone that reports the column after its own
// last one would fire the neighbouring control.
func TestAZoneEndsBeforeItsLastColumn(t *testing.T) {
	var h hitmap
	h.set(0, []zone{{from: 4, to: 8, target: zoneSchema, index: -1}})

	for _, x := range []int{4, 7} {
		if z, ok := h.at(x, 0); !ok || z.target != zoneSchema {
			t.Errorf("column %d is not in the zone that covers 4..8", x)
		}
	}
	for _, x := range []int{3, 8} {
		if _, ok := h.at(x, 0); ok {
			t.Errorf("column %d is in a zone that covers only 4..8", x)
		}
	}
}

// Two regions can draw their headers on the same screen row — the sidebar's
// tab strip and the editor's own header share a row when the sidebar is
// open (buildLayout puts schemaTabs and rightPane in one horizontal Flex).
// set used to replace a row's zones wholesale, so whichever region recorded
// second silently erased the first's.
func TestTwoRegionsSharingARowBothPublishTheirZones(t *testing.T) {
	var h hitmap
	h.set(2, []zone{{from: 1, to: 10, target: zoneTab, index: 0}})
	h.set(2, []zone{{from: 36, to: 80, target: zoneRegionName, index: -1}})

	if z, ok := h.at(5, 2); !ok || z.target != zoneTab {
		t.Errorf("the first region's zone is gone: at(5,2) = %+v, %v", z, ok)
	}
	if z, ok := h.at(40, 2); !ok || z.target != zoneRegionName {
		t.Errorf("the second region's zone is gone: at(40,2) = %+v, %v", z, ok)
	}
}

// The rows a region draws on are its own; a stale row from a region that has
// since been hidden would answer for a click somewhere else entirely.
func TestClearingTheHitmapForgetsEveryRow(t *testing.T) {
	var h hitmap
	h.set(3, []zone{{from: 0, to: 5, target: zoneHelp, index: -1}})
	h.clear()

	if _, ok := h.at(2, 3); ok {
		t.Error("a cleared hitmap still answers for a row it used to hold")
	}
}

// A region draws at its own rect, and the capture is given screen columns.
func TestZonesMoveWithTheRegionTheyWereDrawnIn(t *testing.T) {
	got := offsetZones([]zone{{from: 2, to: 6, target: zoneHelp, index: -1}}, 10)

	if len(got) != 1 || got[0].from != 12 || got[0].to != 16 {
		t.Errorf("offsetZones by 10 = %+v, want from 12 to 16", got)
	}
}
