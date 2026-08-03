package vim

// Entry is one line of the key reference.
type Entry struct {
	// Keys is the sequence exactly as it is typed, so that the reference can
	// be checked against the state machine rather than trusted.
	Keys        string
	Description string
}

// Group is a titled section of the reference.
type Group struct {
	Title   string
	Entries []Entry
}

// Reference lists the modal commands for the help screen and `dv keys`.
//
// It lives beside the state machine on purpose. A reference kept anywhere
// else drifts, and a key reference that lies is worse than none — it is
// consulted precisely when something has already stopped working.
func Reference() []Group {
	return []Group{
		{
			Title: "Modes",
			Entries: []Entry{
				{"i", "insert before the cursor"},
				{"a", "insert after the cursor"},
				{"I", "insert at the first non-blank"},
				{"A", "insert at the end of the line"},
				{"o", "open a line below and insert"},
				{"O", "open a line above and insert"},
				{"v", "start a selection"},
				{"V", "select whole lines"},
				{"<esc>", "back to normal mode"},
			},
		},
		{
			Title: "Moving",
			Entries: []Entry{
				{"h", "left"},
				{"j", "down"},
				{"k", "up"},
				{"l", "right"},
				{"w", "to the next word"},
				{"f{char}", "to the next occurrence of a character"},
				{"t{char}", "to just before it"},
				{"F{char}", "back to a character"},
				{"T{char}", "back to just after it"},
				{"b", "to the previous word"},
				{"e", "to the end of the word"},
				{"0", "to the start of the line"},
				{"^", "to the first non-blank"},
				{"$", "to the end of the line"},
				{"gg", "to the start"},
				{"G", "to the end"},
				{"/", "search forwards"},
				{"?", "search backwards"},
				{"n", "to the next match"},
				{"N", "to the previous match"},
				{"3j", "any motion repeats: a count goes in front"},
			},
		},
		{
			Title: "Changing text",
			Entries: []Entry{
				{"x", "delete a character"},
				{"dw", "delete a word"},
				{"dd", "delete a line"},
				{"D", "delete to the end of the line"},
				{"cw", "change a word"},
				{"ciw", "change the word under the cursor"},
				{"ci(", "change what the brackets hold; also [ { < ' \" `"},
				{"ca(", "and \"a\" takes the brackets too"},
				{"cc", "change a line"},
				{"C", "change to the end of the line"},
				{"yy", "yank a line"},
				{"yw", "yank a word"},
				{"p", "put after the cursor"},
				{"P", "put before the cursor"},
				{".", "repeat the last change"},
				{"u", "undo"},
				{"<c-r>", "redo"},
			},
		},
		{
			Title: "The command line",
			Entries: []Entry{
				// One entry, and it is the colon rather than ":w". What comes
				// after it is typed into a prompt this package never sees, so
				// listing ":w" here would be a sequence the state machine
				// cannot be asked to confirm.
				{":", "run a command: :w saves, :q quits, :e opens a file"},
			},
		},
	}
}
