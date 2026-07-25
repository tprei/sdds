package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/sqlite"
)

func seedReportsDatabase(t *testing.T, databasePath string) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(databasePath)
	if err != nil {
		t.Fatalf("open seed database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close seed database: %v", err)
		}
	}()
	if err := sqlite.ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	const reporter = "u-reporter"
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', 0, 0)`, reporter); err != nil {
		t.Fatalf("insert reporter: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO notes (id, user_id, title, body, category_slug, place_slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"n1", reporter, "Nota boa", "Tem pão de queijo.", note.CategorySlugFood, note.PlaceSlugSaoPaulo, 0, 0); err != nil {
		t.Fatalf("insert note: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO note_comments (id, note_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)`,
		"c1", "n1", reporter, "Comentário legal", 0); err != nil {
		t.Fatalf("insert comment: %v", err)
	}

	reports := []struct {
		id, targetType, targetID, reason string
		details                          string
		hasDetails                       bool
		createdAt                        int64
	}{
		{id: "report-1", targetType: "note", targetID: "n1", reason: "spam", createdAt: 1700000000000},
		{id: "report-2", targetType: "comment", targetID: "c1", reason: "harassment", details: "explicação", hasDetails: true, createdAt: 1700000000001},
		{id: "report-3", targetType: "note", targetID: "gone-note", reason: "other", createdAt: 1700000000002},
	}
	for _, r := range reports {
		var (
			query string
			args  []any
		)
		if r.hasDetails {
			query = `INSERT INTO reports (id, reporter_user_id, target_type, target_id, reason, details, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
			args = []any{r.id, reporter, r.targetType, r.targetID, r.reason, r.details, r.createdAt}
		} else {
			query = `INSERT INTO reports (id, reporter_user_id, target_type, target_id, reason, created_at) VALUES (?, ?, ?, ?, ?, ?)`
			args = []any{r.id, reporter, r.targetType, r.targetID, r.reason, r.createdAt}
		}
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("insert report %q: %v", r.id, err)
		}
	}
}

func captureReportOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	original := reportOutputStream
	buf := &bytes.Buffer{}
	reportOutputStream = buf
	t.Cleanup(func() { reportOutputStream = original })
	return buf
}

type decodedReportRow struct {
	ReportPageKey  int64   `json:"report_page_key"`
	ID             string  `json:"id"`
	CreatedAt      int64   `json:"created_at"`
	ReporterUserID string  `json:"reporter_user_id"`
	TargetType     string  `json:"target_type"`
	TargetID       string  `json:"target_id"`
	Reason         string  `json:"reason"`
	Details        *string `json:"details"`
	TargetSummary  *string `json:"target_summary"`
	TargetMissing  int     `json:"target_missing"`
}

func decodeReportRows(t *testing.T, output string) []decodedReportRow {
	t.Helper()
	rows := make([]decodedReportRow, 0)
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		if line == "" {
			continue
		}
		var row decodedReportRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("decode report line %q: %v", line, err)
		}
		rows = append(rows, row)
	}
	return rows
}

func assertReportRow(
	t *testing.T,
	row decodedReportRow,
	pageKey int64,
	createdAt int64,
	reporter, id, targetType, targetID, reason string,
	details *string,
	summary string,
	missing int,
) {
	t.Helper()
	if row.ReportPageKey != pageKey || row.CreatedAt != createdAt || row.ReporterUserID != reporter {
		t.Fatalf("row = %+v, want page_key=%d created_at=%d reporter=%s", row, pageKey, createdAt, reporter)
	}
	if row.ID != id || row.TargetType != targetType || row.TargetID != targetID || row.Reason != reason {
		t.Fatalf("row = %+v, want id=%s type=%s target=%s reason=%s", row, id, targetType, targetID, reason)
	}
	if row.TargetMissing != missing {
		t.Fatalf("row %s target_missing = %d, want %d", id, row.TargetMissing, missing)
	}
	if missing == 1 {
		if row.TargetSummary != nil {
			t.Fatalf("row %s target summary = %v, want nil for missing target", id, row.TargetSummary)
		}
	} else if row.TargetSummary == nil || *row.TargetSummary != summary {
		t.Fatalf("row %s target summary = %v, want %q", id, row.TargetSummary, summary)
	}
	if details == nil {
		if row.Details != nil {
			t.Fatalf("row %s details = %v, want nil", id, row.Details)
		}
	} else if row.Details == nil || *row.Details != *details {
		t.Fatalf("row %s details = %v, want %q", id, row.Details, *details)
	}
}

func TestRunInspectReportsOutputsReportRows(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "sdds.db")
	seedReportsDatabase(t, databasePath)
	stdout := captureReportOutput(t)

	if err := runInspectReports(ctx, config{databasePath: databasePath}); err != nil {
		t.Fatalf("run inspect reports: %v", err)
	}

	rows := decodeReportRows(t, stdout.String())
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	// Ordered by report_page_key ascending.
	if rows[0].ReportPageKey >= rows[1].ReportPageKey || rows[1].ReportPageKey >= rows[2].ReportPageKey {
		t.Fatalf("rows not ordered by report_page_key: %d %d %d", rows[0].ReportPageKey, rows[1].ReportPageKey, rows[2].ReportPageKey)
	}
	details := "explicação"
	assertReportRow(t, rows[0], 1, 1700000000000, "u-reporter", "report-1", "note", "n1", "spam", nil, "Nota boa", 0)
	assertReportRow(t, rows[1], 2, 1700000000001, "u-reporter", "report-2", "comment", "c1", "harassment", &details, "Comentário legal", 0)
	assertReportRow(t, rows[2], 3, 1700000000002, "u-reporter", "report-3", "note", "gone-note", "other", nil, "", 1)
}

func TestRunInspectReportsEmptyOutputsNothing(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "sdds.db")
	seedReportsDatabase(t, databasePath)

	db, err := sqlite.Open(databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM reports`); err != nil {
		t.Fatalf("delete reports: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	stdout := captureReportOutput(t)
	if err := runInspectReports(ctx, config{databasePath: databasePath}); err != nil {
		t.Fatalf("run inspect reports: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("inspect output = %q, want empty", stdout.String())
	}
}

func TestRunInspectReportsFailsOnMissingDatabase(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "does-not-exist", "sdds.db")
	stdout := captureReportOutput(t)

	if err := runInspectReports(ctx, config{databasePath: databasePath}); err == nil {
		t.Fatal("run inspect reports on missing database error = nil, want error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("inspect output on failure = %q, want empty (no partial stdout)", stdout.String())
	}
}
