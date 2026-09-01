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
type hitmap struct {
	rows map[int][]zone
}

func (h *hitmap) set(row int, zones []zone) {
	if h.rows == nil {
		h.rows = make(map[int][]zone)
	}
	if len(zones) == 0 {
		delete(h.rows, row)
		return
	}
	h.rows[row] = zones
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
