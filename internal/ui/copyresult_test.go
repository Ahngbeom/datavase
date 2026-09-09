package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/result"
)

func TestResultTextFollowsTheDisplayedOrder(t *testing.T) {
	buf := result.NewBuffer(10)
	buf.SetColumns([]string{"n"}, nil)
	buf.Append([][]any{{int64(3)}, {int64(1)}, {int64(2)}})
	content := newGridContent(buf)
	content.sortBy(0) // ascending

	text, err := resultText(buf, content, formatMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	want := "| n |\n| --- |\n| 1 |\n| 2 |\n| 3 |\n"
	if text != want {
		t.Errorf("resultText() =\n%s\nwant the sorted order\n%s", text, want)
	}
}

func TestResultTextAsJSONIsAnArrayOfObjects(t *testing.T) {
	buf := result.NewBuffer(10)
	buf.SetColumns([]string{"id"}, nil)
	buf.Append([][]any{{int64(7)}})

	text, err := resultText(buf, newGridContent(buf), formatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, `"id": 7`) {
		t.Errorf("resultText() = %q, want JSON with the row", text)
	}
}

func TestCopySummarySaysWhenTheBufferWasCut(t *testing.T) {
	got := copySummary(3, formatMarkdown, 120, true)
	for _, want := range []string{"3 rows", "Markdown", "120 B", "truncated"} {
		if !strings.Contains(got, want) {
			t.Errorf("copySummary() = %q, want it to mention %q", got, want)
		}
	}
	if got := copySummary(1, formatJSON, 2048, false); strings.Contains(got, "truncated") || !strings.Contains(got, "2.0 KB") {
		t.Errorf("copySummary() = %q", got)
	}
}
