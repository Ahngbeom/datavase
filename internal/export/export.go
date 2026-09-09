// Package export writes result sets to CSV, JSON and Markdown.
//
// Export formatting is deliberately separate from the grid's: the grid
// abbreviates for the eye — "NULL" for absence, an ellipsis for long values —
// while an export has to be faithful, because something else will read it.
package export

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// CSV writes columns and rows as comma-separated values.
func CSV(w io.Writer, columns []string, rows [][]any) error {
	out := csv.NewWriter(w)

	if err := out.Write(columns); err != nil {
		return fmt.Errorf("writing the header: %w", err)
	}

	record := make([]string, len(columns))
	for _, row := range rows {
		for i := range record {
			if i < len(row) {
				record[i] = csvField(row[i])
			} else {
				record[i] = ""
			}
		}
		if err := out.Write(record); err != nil {
			return fmt.Errorf("writing a row: %w", err)
		}
	}

	out.Flush()
	return out.Error()
}

// csvField renders one value for CSV.
func csvField(v any) string {
	if v == nil {
		// An empty field, not the word NULL: a column really containing
		// "NULL" has to stay distinguishable from an absent value.
		return ""
	}
	return neutraliseFormula(plainText(v))
}

// formulaLeaders are the characters a spreadsheet treats as starting a
// formula.
const formulaLeaders = "=+-@\t\r"

// neutraliseFormula stops a spreadsheet from executing an exported value.
//
// Opening a CSV in Excel or Sheets runs any field beginning with =, +, - or @
// as a formula, so a row from the database becomes code on someone's machine.
// Prefixing an apostrophe is the conventional defence: spreadsheets read it
// as "this is text" and do not display it, while plain readers see it and can
// strip it.
func neutraliseFormula(s string) string {
	if s == "" || !strings.ContainsRune(formulaLeaders, rune(s[0])) {
		return s
	}
	return "'" + s
}

// JSON writes columns and rows as an array of objects.
func JSON(w io.Writer, columns []string, rows [][]any) error {
	names := uniqueNames(columns)

	objects := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		object := make(map[string]any, len(names))
		for i, name := range names {
			if i < len(row) {
				object[name] = jsonValue(row[i])
			} else {
				object[name] = nil
			}
		}
		objects = append(objects, object)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(objects)
}

// uniqueNames makes duplicate column names distinct.
//
// "SELECT a.id, b.id FROM …" is ordinary SQL but cannot be a JSON object, and
// silently keeping only the last would lose data the user asked for.
func uniqueNames(columns []string) []string {
	seen := make(map[string]int, len(columns))
	out := make([]string, len(columns))

	for i, name := range columns {
		count := seen[name]
		seen[name] = count + 1
		if count == 0 {
			out[i] = name
			continue
		}
		out[i] = name + "_" + strconv.Itoa(count+1)
	}
	return out
}

// jsonValue converts a scanned value to something the encoder can render
// with its own type, so numbers stay numbers for jq and friends.
func jsonValue(v any) any {
	switch value := v.(type) {
	case nil:
		return nil
	case []byte:
		if utf8.Valid(value) {
			return string(value)
		}
		// Binary has no textual form; base64 keeps it recoverable rather
		// than corrupting it into replacement characters.
		return base64.StdEncoding.EncodeToString(value)
	case time.Time:
		return value.Format(time.RFC3339)
	default:
		return v
	}
}

// plainText renders a value for CSV and Markdown: faithful, never abbreviated.
func plainText(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case []byte:
		if utf8.Valid(value) {
			return string(value)
		}
		return base64.StdEncoding.EncodeToString(value)
	case string:
		return value
	case bool:
		if value {
			return "1"
		}
		return "0"
	case int64:
		return strconv.FormatInt(value, 10)
	case uint64:
		return strconv.FormatUint(value, 10)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32)
	case time.Time:
		return value.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprint(v)
	}
}

// Markdown writes columns and rows as a pipe table.
//
// A table with no columns is written as nothing at all: "|  |" is not a
// table, and a paste that contains it looks like a bug rather than an empty
// result.
func Markdown(w io.Writer, columns []string, rows [][]any) error {
	if len(columns) == 0 {
		return nil
	}

	var b strings.Builder
	writeRow := func(cells []string) {
		b.WriteString("| ")
		b.WriteString(strings.Join(cells, " | "))
		b.WriteString(" |\n")
	}

	header := make([]string, len(columns))
	separator := make([]string, len(columns))
	for i, name := range columns {
		header[i] = markdownCell(name)
		separator[i] = "---"
	}
	writeRow(header)
	writeRow(separator)

	cells := make([]string, len(columns))
	for _, row := range rows {
		for i := range cells {
			cells[i] = ""
			if i < len(row) {
				cells[i] = markdownCell(plainText(row[i]))
			}
		}
		writeRow(cells)
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// markdownCell keeps a value inside its cell: a pipe would start a new
// column and a newline a new row, and either silently misaligns everything
// after it.
func markdownCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\r\n", "<br>")
	return strings.ReplaceAll(s, "\n", "<br>")
}
