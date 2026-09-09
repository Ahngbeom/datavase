package ui

import "testing"

func TestPreviewSQLQuotesBothNamesAndLimitsTo100(t *testing.T) {
	got := previewSQL("app db", "order-items")
	want := "SELECT * FROM `app db`.`order-items` LIMIT 100"
	if got != want {
		t.Errorf("previewSQL() = %q, want %q", got, want)
	}
}
