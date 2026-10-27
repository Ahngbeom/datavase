package headless

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/export"
	"github.com/Ahngbeom/datavase/internal/sqlparse"
)

// Limits bound what one statement may cost.
type Limits struct {
	// MaxRows is how many rows are written at most.
	MaxRows int
	// Timeout is how long the statement may run before the server is asked
	// to end it.
	Timeout time.Duration
}

// Run sends stmt on conn and writes its result to w as JSON lines: a header
// object, one array per row, and a summary object.
//
// The summary is written only when the result was read to its end or to the
// cap. A reader that finds no summary has an incomplete result, whatever
// rows came before it, and the error returned says why.
func Run(ctx context.Context, conn *db.Conn, stmt sqlparse.Statement, lim Limits, w io.Writer) error {
	if !conn.ReadOnlyConfirmed() {
		return refuse("%s is not a read_only datasource the server has confirmed; dv query runs nothing on it",
			conn.DataSource().Name)
	}
	if lim.MaxRows <= 0 || lim.Timeout <= 0 {
		return errors.New("headless: MaxRows and Timeout must both be positive")
	}

	// One more than the caller wants, so that a result of exactly MaxRows
	// rows is not reported as cut short.
	fetch := lim.MaxRows + 1
	query := sqlparse.AppendLimit(stmt, sqlparse.AutoLimit(stmt, fetch))

	started := time.Now()
	stream := conn.Query(ctx, query, db.Options{MaxRows: fetch})
	defer stream.Close()

	// Cancel rather than a context deadline: a deadline only detaches the
	// client, and the server goes on running the statement for nobody.
	// Cancel sends KILL QUERY on the control connection.
	var timedOut atomic.Bool
	timer := time.AfterFunc(lim.Timeout, func() {
		timedOut.Store(true)
		stream.Cancel()
	})
	defer timer.Stop()
	defer context.AfterFunc(ctx, func() { stream.Cancel() })()

	out := newLines(w, lim.MaxRows)
	var writeErr error
	for ev := range stream.Events {
		// Keep draining after a failed write: the stream ends only when its
		// channel closes, and walking away would leave the statement running.
		if writeErr != nil {
			continue
		}
		switch ev.Kind {
		case db.EventColumns:
			writeErr = out.header(ev.Columns, typeNames(ev.Types))
		case db.EventRows:
			writeErr = out.add(ev.Rows)
		}
		if writeErr != nil {
			stream.Cancel()
		}
	}

	if timedOut.Load() {
		return fmt.Errorf("stopped after %s; the server was asked to end the statement (KILL QUERY)", lim.Timeout)
	}
	if err := stream.Err(); err != nil {
		return err
	}
	if writeErr != nil {
		return fmt.Errorf("writing the result: %w", writeErr)
	}
	return out.summary(time.Since(started))
}

func typeNames(types []*sql.ColumnType) []string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.DatabaseTypeName()
	}
	return names
}

// lines writes the result. The header and the summary are objects and every
// row is an array, so a reader can tell the three apart by the first byte.
type lines struct {
	enc       *json.Encoder
	max       int
	written   int
	truncated bool
}

func newLines(w io.Writer, max int) *lines {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &lines{enc: enc, max: max}
}

func (l *lines) header(columns, types []string) error {
	return l.enc.Encode(struct {
		Columns []string `json:"columns"`
		Types   []string `json:"types"`
	}{columns, types})
}

func (l *lines) add(rows [][]any) error {
	for _, row := range rows {
		if l.written >= l.max {
			l.truncated = true
			return nil
		}
		cells := make([]any, len(row))
		for i, v := range row {
			cells[i] = export.JSONValue(v)
		}
		if err := l.enc.Encode(cells); err != nil {
			return err
		}
		l.written++
	}
	return nil
}

func (l *lines) summary(elapsed time.Duration) error {
	return l.enc.Encode(struct {
		Rows      int   `json:"rows"`
		Truncated bool  `json:"truncated"`
		ElapsedMS int64 `json:"elapsed_ms"`
	}{l.written, l.truncated, elapsed.Milliseconds()})
}
