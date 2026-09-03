package ui

// zone is a run of columns on one row that means something when clicked.
//
// Regions here render a line that sheds fields until it fits, so which
// columns hold what is known only to the renderer and only for the width it
// was asked about. A zone is that knowledge, produced in the same pass as the
// string: a degradation that drops a field drops its zone with it, which is
// the only arrangement in which a click cannot land on whatever moved into
// those columns.
type zone struct {
	// from and to are columns, half-open.
	from, to int
	target   zoneTarget
	// index distinguishes members of a repeated target — which tab. It is
	// -1 for the targets that have only one instance.
	index int
}

// zoneTarget is what a click on a zone asks for.
//
// The environment is not here. It is the one visual thing standing between
// the user and a production mistake, and a warning that is also a control
// means a misclick on it looks like it changed the environment. Switching
// datasource is on the name immediately beside it.
type zoneTarget int

const (
	zoneNone zoneTarget = iota
	zoneDataSource
	zoneSchema
	zoneHelp
	zoneTab
	zoneRegionName
	zoneStatusMode
	zoneStatusWrites
)

// hitmap is the zones of the last frame, by screen row.
//
// It is rebuilt every frame rather than updated, for the reason the region
// headers are: the only way it can be wrong is if a renderer is, and there is
// no update to forget.
//
// Nothing here takes a lock, which is safe only because tview never draws
// off the goroutine running Application.Run — and that is a narrower
// guarantee than it sounds: tview's own poll goroutine draws directly,
// bypassing the event loop, if SetScreen is called again on an already-
// running Application. dv never takes that path. daemon/serve.go calls
// SetScreen exactly once, before the session's Run starts; a re-attach
// reuses that same screen through Screen.Attach and Screen.Detach rather
// than handing tview a new one. A future change that replaces the screen on
// re-attach — an obvious thing to reach for — would put a draw on a second
// goroutine and make every read and write here a race, silently rather than
// loudly, since a race like this one shows up as an occasional wrong click
// rather than a crash.
type hitmap struct {
	rows map[int][]zone
}

// set adds a region's zones to a row, rather than replacing whatever another
// region already recorded there this frame.
//
// buildLayout can put two regions' headers on the same screen row — the
// sidebar's tab strip and the editor's own header both land on the body's
// first row when the sidebar is open — and each records independently of
// the other. Replacing wholesale meant only the last of the two to draw
// published anything at all; appending keeps both, in the order they were
// recorded, which is what at's first-match scan relies on. This is safe
// only because clear wipes every row once per frame (captureScreen's
// SetBeforeDrawFunc) before any region records again — appending within
// that single frame cannot accumulate a stale zone from the one before.
func (h *hitmap) set(row int, zones []zone) {
	if len(zones) == 0 {
		return
	}
	if h.rows == nil {
		h.rows = make(map[int][]zone)
	}
	h.rows[row] = append(h.rows[row], zones...)
}

func (h *hitmap) clear() { h.rows = nil }

// at answers which zone covers a screen position, if any.
func (h *hitmap) at(x, y int) (zone, bool) {
	for _, z := range h.rows[y] {
		if x >= z.from && x < z.to {
			return z, true
		}
	}
	return zone{}, false
}

// offsetZones moves zones from a region's own columns into the screen's.
func offsetZones(zones []zone, dx int) []zone {
	if dx == 0 || len(zones) == 0 {
		return zones
	}
	out := make([]zone, len(zones))
	for i, z := range zones {
		z.from += dx
		z.to += dx
		out[i] = z
	}
	return out
}
