package ui

import "strings"

// Each region's header carries what can be done in it, and only while the
// keyboard is there.
//
// The header is the one line a region already owns, and the results tab was
// already using it to offer the copy key. The other two left it blank, so
// everything they could do — completion, the history, the preview a
// double-click opens — existed with nothing on screen to say so.
//
// Only the focused region speaks. Three headers offering keys at once is a
// screen nobody reads, and the reader who has learned to skip them misses the
// one that mattered. Moving between panes is how the rest is found: each stop
// says what it can do, once.

// affordanceKeys are the labels for the keys a hint names, taken from the map
// in force so a rebinding cannot leave a hint advertising a dead key.
type affordanceKeys struct {
	run, runAll, complete, history string
	copy, sort, inspect, row       string
}

// joinHints puts a middle dot between the parts, skipping the empty ones.
func joinHints(parts ...string) string {
	kept := parts[:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " · ")
}

// editorAffordance is what the editor can do beyond taking the typing, which
// is the part a reader can see for themselves.
//
// Two, not more: on an eighty-column terminal the region has room for about
// thirty cells before the header drops the detail whole, and a hint that
// disappears on the narrowest screen helps nobody. Run is why the text is
// being typed. Completion is the one no reader guesses — Ctrl+Space announces
// itself nowhere — while the history and the shifted run key are a keystroke
// from the reference this line points at.
func editorAffordance(focused bool, k affordanceKeys) string {
	if !focused {
		return ""
	}
	return joinHints(k.run+" run", k.complete+" complete")
}

// treeAffordance names the preview, which is the tree's reason for being
// clickable at all and had nothing anywhere to announce it.
//
// It names the gesture rather than a key because there is no key for it: the
// mouse is the whole thing, and a reader hunting the keyboard would not find
// it. Three words, because the schema pane is thirty-four columns wide and
// the tab strip has most of them.
func treeAffordance(focused bool, k affordanceKeys) string {
	if !focused {
		return ""
	}
	return "dbl-click previews"
}

// gridAffordance offers the three things a result can do, once there is a
// result. An empty grid naming them would be three keys that answer nothing.
func gridAffordance(focused, hasRows bool, k affordanceKeys) string {
	if !focused || !hasRows {
		return ""
	}
	return joinHints(k.copy+" copy", k.sort+" sort", k.inspect+" row")
}

// failureDirection is where to look in this window after a failure the
// interface can actually help with, or empty for the rest.
//
// It is returned apart from the server's message rather than appended to it
// so the status bar can carry them as two fields: the message is the fact and
// never sheds, the direction is a convenience and goes first when the line is
// too narrow for both. Appending would have made a narrow terminal truncate
// the fact to make room for the advice.
//
// A missing table is the case worth answering: what the reader wants is on
// screen already, in the tree, and the server's message does not mention it.
func failureDirection(message string) string {
	if strings.Contains(message, "doesn't exist") || strings.Contains(message, "Unknown database") {
		return "the tree lists what is there"
	}
	return ""
}
