package ui

// confirmDiscard asks before an action that throws work away.
//
// There is a way through, because this is the user's own work — the point is
// that it takes a decision rather than a reflex.
func (a *App) confirmDiscard(text, proceedLabel string, proceed func()) {
	modal := newModal().
		SetText(text).
		AddButtons([]string{"Cancel", proceedLabel}).
		SetDoneFunc(func(_ int, label string) {
			a.closeDialog()
			if label == proceedLabel {
				proceed()
			}
		})

	modal.SetTextColor(colourNotice)
	a.openDialog(modal)
}
