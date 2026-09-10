package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestResultTextAsCSVFollowsTheDisplayedOrder(t *testing.T) {
	buf := result.NewBuffer(10)
	buf.SetColumns([]string{"n"}, nil)
	buf.Append([][]any{{int64(3)}, {int64(1)}, {int64(2)}})
	content := newGridContent(buf)
	content.sortBy(0)

	text, err := resultText(buf, content, formatCSV)
	if err != nil {
		t.Fatal(err)
	}
	if want := "n\n1\n2\n3\n"; text != want {
		t.Errorf("resultText() = %q, want the sorted CSV %q", text, want)
	}
}

func TestWritingAResultRefusesToOverwriteAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.csv")
	if err := writeResultFile(path, "first\n"); err != nil {
		t.Fatalf("first write: %v", err)
	}
	err := writeResultFile(path, "second\n")
	if err == nil {
		t.Fatal("the second write replaced a file that was already there")
	}
	if !strings.Contains(err.Error(), "exists") {
		t.Errorf("err = %v, want it to say the file exists", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "first\n" {
		t.Errorf("file = %q, the refused write still changed it", got)
	}
}

func TestWritingAResultExpandsTheHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := writeResultFile("~/out.csv", "x\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "out.csv")); err != nil {
		t.Errorf("~ was not expanded: %v", err)
	}
}

func TestDefaultResultPathNamesTheDatasourceAndTheMoment(t *testing.T) {
	at := time.Date(2026, 9, 10, 15, 30, 12, 0, time.Local)
	if got, want := defaultResultPath("app", at), "app-20260910-153012.csv"; got != want {
		t.Errorf("defaultResultPath() = %q, want %q", got, want)
	}
}

func TestSaveSummaryNamesTheFile(t *testing.T) {
	got := saveSummary(3, "/tmp/x.csv", 120, true)
	for _, want := range []string{"3 rows", "/tmp/x.csv", "120 B", "truncated"} {
		if !strings.Contains(got, want) {
			t.Errorf("saveSummary() = %q, want it to mention %q", got, want)
		}
	}
}
