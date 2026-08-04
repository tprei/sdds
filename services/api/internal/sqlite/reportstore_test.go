package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/report"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	reportStoreReporterUserID  = user.UserID("018ff5b8-0000-7000-8000-000000000201")
	reportStoreOtherReporterID = user.UserID("018ff5b8-0000-7000-8000-000000000202")
	reportStoreBareUserID      = user.UserID("018ff5b8-0000-7000-8000-000000000203")
	reportStoreNoteID          = "018ff5b8-0000-7000-8000-000000000210"
	reportStoreOtherNoteID     = "018ff5b8-0000-7000-8000-000000000211"
	reportStoreCommentID       = "018ff5b8-0000-7000-8000-000000000220"
)

func TestReportStoreCreatesReportForEachReason(t *testing.T) {
	ctx := context.Background()
	store, db := newReportStoreTestStore(t, ctx)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreReporterUserID)
	insertAuthorStoreNote(t, ctx, db, reportStoreNoteID, reportStoreReporterUserID, 0)

	for _, reason := range []report.Reason{report.ReasonSpam, report.ReasonHarassment, report.ReasonHarmfulOrMisleading, report.ReasonOther} {
		targetID := "note-" + string(reason)
		insertAuthorStoreNote(t, ctx, db, targetID, reportStoreReporterUserID, 0)
		result, err := store.CreateReport(ctx, report.CreateInput{
			TargetType:     report.TargetTypeNote,
			TargetID:       targetID,
			Reason:         reason,
			ReporterUserID: reportStoreReporterUserID,
		})
		if err != nil {
			t.Fatalf("create %q report: %v", reason, err)
		}
		if !result.Created {
			t.Fatalf("reason %q Created = false, want true", reason)
		}
		if result.Report.Reason != reason {
			t.Fatalf("reason %q stored reason = %q", reason, result.Report.Reason)
		}
		if result.Report.TargetType != report.TargetTypeNote || result.Report.TargetID != targetID {
			t.Fatalf("reason %q target = (%q,%q)", reason, result.Report.TargetType, result.Report.TargetID)
		}
		if result.Report.ReporterUserID != reportStoreReporterUserID {
			t.Fatalf("reason %q reporter = %q", reason, result.Report.ReporterUserID)
		}
		if _, err := uuid.Parse(string(result.Report.ID)); err != nil {
			t.Fatalf("reason %q id parse: %v", reason, err)
		}
		if result.Report.CreatedAt != normalizeTime(result.Report.CreatedAt) {
			t.Fatalf("reason %q created_at not millisecond-normalized", reason)
		}
	}
}

func TestReportStoreDetailsBoundaries(t *testing.T) {
	ctx := context.Background()
	store, db := newReportStoreTestStore(t, ctx)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreReporterUserID)
	insertAuthorStoreNote(t, ctx, db, reportStoreNoteID, reportStoreReporterUserID, 0)

	t.Run("nil details stays nil", func(t *testing.T) {
		result, err := store.CreateReport(ctx, report.CreateInput{
			TargetType: report.TargetTypeNote, TargetID: "nil-note", Reason: report.ReasonSpam,
			ReporterUserID: reportStoreReporterUserID,
		})
		if err != nil {
			t.Fatalf("create nil-details report: %v", err)
		}
		if result.Report.Details != nil {
			t.Fatalf("details = %v, want nil", result.Report.Details)
		}
	})

	t.Run("whitespace details becomes nil", func(t *testing.T) {
		blank := "   \t\n   "
		result, err := store.CreateReport(ctx, report.CreateInput{
			TargetType: report.TargetTypeNote, TargetID: "blank-note", Reason: report.ReasonSpam,
			Details: &blank, ReporterUserID: reportStoreReporterUserID,
		})
		if err != nil {
			t.Fatalf("create blank-details report: %v", err)
		}
		if result.Report.Details != nil {
			t.Fatalf("details = %v, want nil", result.Report.Details)
		}
	})

	t.Run("exactly 1000 code points persist", func(t *testing.T) {
		insertAuthorStoreNote(t, ctx, db, "max-note", reportStoreReporterUserID, 0)
		details := strings.Repeat("é", report.DetailsMaxLength)
		result, err := store.CreateReport(ctx, report.CreateInput{
			TargetType: report.TargetTypeNote, TargetID: "max-note", Reason: report.ReasonSpam,
			Details: &details, ReporterUserID: reportStoreReporterUserID,
		})
		if err != nil {
			t.Fatalf("create 1000-details report: %v", err)
		}
		if result.Report.Details == nil || *result.Report.Details != details {
			t.Fatalf("details = %v, want %q", result.Report.Details, details)
		}
	})

	t.Run("1001 code points rejected", func(t *testing.T) {
		insertAuthorStoreNote(t, ctx, db, "over-note", reportStoreReporterUserID, 0)
		details := strings.Repeat("é", report.DetailsMaxLength+1)
		_, err := store.CreateReport(ctx, report.CreateInput{
			TargetType: report.TargetTypeNote, TargetID: "over-note", Reason: report.ReasonSpam,
			Details: &details, ReporterUserID: reportStoreReporterUserID,
		})
		if err == nil {
			t.Fatal("create 1001-details report error = nil, want validation error")
		}
	})
}

func TestReportStoreTreatsSQLInjectionAsData(t *testing.T) {
	ctx := context.Background()
	store, db := newReportStoreTestStore(t, ctx)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreReporterUserID)

	for _, payload := range []string{"' OR 1=1; --", `"; DROP TABLE reports; --`, "/* 👩‍💻 */"} {
		insertAuthorStoreNote(t, ctx, db, payload, reportStoreReporterUserID, 0)
		details := payload
		result, err := store.CreateReport(ctx, report.CreateInput{
			TargetType: report.TargetTypeNote, TargetID: "  " + payload + "  ", Reason: report.ReasonSpam,
			Details: &details, ReporterUserID: reportStoreReporterUserID,
		})
		if err != nil {
			t.Fatalf("create injection report %q: %v", payload, err)
		}
		if result.Report.TargetID != payload {
			t.Fatalf("target id = %q, want %q", result.Report.TargetID, payload)
		}
		if result.Report.Details == nil || *result.Report.Details != payload {
			t.Fatalf("details = %v, want %q", result.Report.Details, payload)
		}
	}

	var tableCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'reports'`).Scan(&tableCount); err != nil {
		t.Fatalf("check reports table exists: %v", err)
	}
	if tableCount != 1 {
		t.Fatalf("reports table count = %d, want 1 (DROP must not have executed)", tableCount)
	}
}

func TestReportStoreNoteAndCommentTargetsAreDistinct(t *testing.T) {
	ctx := context.Background()
	store, db := newReportStoreTestStore(t, ctx)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreReporterUserID)
	insertAuthorStoreNote(t, ctx, db, reportStoreNoteID, reportStoreReporterUserID, 0)
	if _, err := db.ExecContext(ctx, `INSERT INTO note_comments (id, note_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)`, reportStoreCommentID, reportStoreNoteID, reportStoreReporterUserID, "comentário", 0); err != nil {
		t.Fatalf("insert comment: %v", err)
	}

	noteReport, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreNoteID, Reason: report.ReasonSpam, ReporterUserID: reportStoreReporterUserID,
	})
	if err != nil {
		t.Fatalf("create note report: %v", err)
	}
	commentReport, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeComment, TargetID: reportStoreCommentID, Reason: report.ReasonSpam, ReporterUserID: reportStoreReporterUserID,
	})
	if err != nil {
		t.Fatalf("create comment report: %v", err)
	}
	if !noteReport.Created || !commentReport.Created {
		t.Fatal("distinct targets should both be created")
	}
	if noteReport.Report.ID == commentReport.Report.ID {
		t.Fatal("distinct targets share an id")
	}
	if got := countReports(t, ctx, db); got != 2 {
		t.Fatalf("report count = %d, want 2", got)
	}
}

func TestReportStoreOverwriteKeepsIDAndCreatedAt(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	store := newReportStore(db, func() time.Time { return now })
	insertBareUsefulStoreUser(t, ctx, db, reportStoreReporterUserID)
	insertAuthorStoreNote(t, ctx, db, reportStoreNoteID, reportStoreReporterUserID, 0)

	first, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreNoteID, Reason: report.ReasonSpam, ReporterUserID: reportStoreReporterUserID,
	})
	if err != nil {
		t.Fatalf("create first report: %v", err)
	}
	if !first.Created {
		t.Fatal("first report Created = false, want true")
	}
	if first.Report.Details != nil {
		t.Fatalf("first details = %v, want nil", first.Report.Details)
	}

	now = now.Add(time.Hour)
	changedDetails := "mudou o motivo"
	second, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreNoteID, Reason: report.ReasonHarassment,
		Details: &changedDetails, ReporterUserID: reportStoreReporterUserID,
	})
	if err != nil {
		t.Fatalf("overwrite report: %v", err)
	}
	if second.Created {
		t.Fatal("overwrite Created = true, want false")
	}
	if second.Report.ID != first.Report.ID {
		t.Fatalf("id changed: first = %q, second = %q", first.Report.ID, second.Report.ID)
	}
	if second.Report.CreatedAt != first.Report.CreatedAt {
		t.Fatalf("created_at changed: first = %s, second = %s", first.Report.CreatedAt, second.Report.CreatedAt)
	}
	if second.Report.Reason != report.ReasonHarassment {
		t.Fatalf("reason = %q, want %q", second.Report.Reason, report.ReasonHarassment)
	}
	if second.Report.Details == nil || *second.Report.Details != "mudou o motivo" {
		t.Fatalf("details = %v, want %q", second.Report.Details, "mudou o motivo")
	}
	if got := countReports(t, ctx, db); got != 1 {
		t.Fatalf("report count after overwrite = %d, want 1", got)
	}

	storedID, storedCreatedAt, storedReason, storedDetails, detailsValid := rawReportRow(t, ctx, db, reportStoreReporterUserID, "note", reportStoreNoteID)
	if storedID != string(first.Report.ID) {
		t.Fatalf("stored id = %q, want %q", storedID, first.Report.ID)
	}
	if timeFromUnixMillis(storedCreatedAt) != first.Report.CreatedAt {
		t.Fatalf("stored created_at = %d, want %d", storedCreatedAt, first.Report.CreatedAt.UnixMilli())
	}
	if storedReason != "harassment" {
		t.Fatalf("stored reason = %q, want harassment", storedReason)
	}
	if !detailsValid || storedDetails != "mudou o motivo" {
		t.Fatalf("stored details = %q (valid=%v), want %q", storedDetails, detailsValid, "mudou o motivo")
	}
}

func TestReportStoreOverwriteCanClearDetails(t *testing.T) {
	ctx := context.Background()
	store, db := newReportStoreTestStore(t, ctx)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreReporterUserID)
	insertAuthorStoreNote(t, ctx, db, reportStoreNoteID, reportStoreReporterUserID, 0)

	withDetails := "explicação"
	if _, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreNoteID, Reason: report.ReasonSpam,
		Details: &withDetails, ReporterUserID: reportStoreReporterUserID,
	}); err != nil {
		t.Fatalf("create with details: %v", err)
	}

	blank := "   "
	second, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreNoteID, Reason: report.ReasonOther,
		Details: &blank, ReporterUserID: reportStoreReporterUserID,
	})
	if err != nil {
		t.Fatalf("overwrite clearing details: %v", err)
	}
	if second.Created {
		t.Fatal("overwrite Created = true, want false")
	}
	if second.Report.Details != nil {
		t.Fatalf("details = %v, want nil after clearing", second.Report.Details)
	}
	_, _, _, _, detailsValid := rawReportRow(t, ctx, db, reportStoreReporterUserID, "note", reportStoreNoteID)
	if detailsValid {
		t.Fatal("stored details present, want NULL")
	}
}

func TestReportStoreSeparateReportersAndTargetsAreSeparateRows(t *testing.T) {
	ctx := context.Background()
	store, db := newReportStoreTestStore(t, ctx)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreReporterUserID)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreOtherReporterID)
	insertAuthorStoreNote(t, ctx, db, reportStoreNoteID, reportStoreReporterUserID, 0)
	insertAuthorStoreNote(t, ctx, db, reportStoreOtherNoteID, reportStoreReporterUserID, 0)

	first, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreNoteID, Reason: report.ReasonSpam, ReporterUserID: reportStoreReporterUserID,
	})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}

	sameReporterOtherTarget, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreOtherNoteID, Reason: report.ReasonSpam, ReporterUserID: reportStoreReporterUserID,
	})
	if err != nil {
		t.Fatalf("create same reporter other target: %v", err)
	}
	if !sameReporterOtherTarget.Created {
		t.Fatal("same reporter, different target should be created")
	}

	otherReporterSameTarget, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreNoteID, Reason: report.ReasonSpam, ReporterUserID: reportStoreOtherReporterID,
	})
	if err != nil {
		t.Fatalf("create other reporter same target: %v", err)
	}
	if !otherReporterSameTarget.Created {
		t.Fatal("different reporter, same target should be created")
	}

	if got := countReports(t, ctx, db); got != 3 {
		t.Fatalf("report count = %d, want 3", got)
	}
	if first.Report.ID == sameReporterOtherTarget.Report.ID || first.Report.ID == otherReporterSameTarget.Report.ID {
		t.Fatal("separate rows share an id")
	}
}

func TestReportStoreRejectsEmptyReporter(t *testing.T) {
	ctx := context.Background()
	store, db := newReportStoreTestStore(t, ctx)

	if _, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreNoteID, Reason: report.ReasonSpam, ReporterUserID: "",
	}); err == nil {
		t.Fatal("create with empty reporter error = nil, want defensive error")
	}
	if got := countReports(t, ctx, db); got != 0 {
		t.Fatalf("report count = %d, want 0", got)
	}
}

func TestReportStoreDirectInvalidInsertsFail(t *testing.T) {
	ctx := context.Background()
	_, db := newReportStoreTestStore(t, ctx)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreReporterUserID)

	inserts := []struct {
		name    string
		columns string
		values  []any
	}{
		{name: "invalid reason", columns: "id, reporter_user_id, target_type, target_id, reason, created_at", values: []any{uuid.NewString(), reportStoreReporterUserID, "note", "n1", "bogus", 0}},
		{name: "invalid target type", columns: "id, reporter_user_id, target_type, target_id, reason, created_at", values: []any{uuid.NewString(), reportStoreReporterUserID, "account", "n1", "spam", 0}},
		{name: "blank target id", columns: "id, reporter_user_id, target_type, target_id, reason, created_at", values: []any{uuid.NewString(), reportStoreReporterUserID, "note", "  ", "spam", 0}},
		{name: "empty details", columns: "id, reporter_user_id, target_type, target_id, reason, details, created_at", values: []any{uuid.NewString(), reportStoreReporterUserID, "note", "n1", "spam", "  ", 0}},
		{name: "too long details", columns: "id, reporter_user_id, target_type, target_id, reason, details, created_at", values: []any{uuid.NewString(), reportStoreReporterUserID, "note", "n1", "spam", strings.Repeat("a", 1001), 0}},
	}
	for _, insert := range inserts {
		t.Run(insert.name, func(t *testing.T) {
			placeholders := strings.Repeat("?, ", len(insert.values))
			placeholders = "(" + strings.TrimSuffix(placeholders, ", ") + ")"
			query := "INSERT INTO reports (" + insert.columns + ") VALUES " + placeholders
			if _, err := db.ExecContext(ctx, query, insert.values...); err == nil {
				t.Fatalf("direct %q insert succeeded, want CHECK error", insert.name)
			}
		})
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO reports (id, reporter_user_id, target_type, target_id, reason, created_at) VALUES (?, ?, ?, ?, ?, ?)`, uuid.NewString(), "ghost-user", "note", "n1", "spam", 0); err == nil {
		t.Fatal("direct missing-reporter insert succeeded, want foreign key error")
	}
}

func TestReportStoreReporterDeletionCascades(t *testing.T) {
	ctx := context.Background()
	store, db := newReportStoreTestStore(t, ctx)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreBareUserID)

	if _, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: "orphan-note", Reason: report.ReasonSpam, ReporterUserID: reportStoreBareUserID,
	}); err != nil {
		t.Fatalf("create report: %v", err)
	}
	if got := countReports(t, ctx, db); got != 1 {
		t.Fatalf("report count before delete = %d, want 1", got)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, reportStoreBareUserID); err != nil {
		t.Fatalf("delete reporter: %v", err)
	}
	if got := countReports(t, ctx, db); got != 0 {
		t.Fatalf("report count after reporter delete = %d, want 0 (cascade)", got)
	}
}

func TestReportStoreTargetDeletionLeavesReport(t *testing.T) {
	ctx := context.Background()
	store, db := newReportStoreTestStore(t, ctx)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreReporterUserID)
	insertAuthorStoreNote(t, ctx, db, reportStoreNoteID, reportStoreReporterUserID, 0)
	if _, err := db.ExecContext(ctx, `INSERT INTO note_comments (id, note_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)`, reportStoreCommentID, reportStoreNoteID, reportStoreReporterUserID, "comentário", 0); err != nil {
		t.Fatalf("insert comment: %v", err)
	}

	if _, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreNoteID, Reason: report.ReasonSpam, ReporterUserID: reportStoreReporterUserID,
	}); err != nil {
		t.Fatalf("create note report: %v", err)
	}
	if _, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeComment, TargetID: reportStoreCommentID, Reason: report.ReasonHarassment, ReporterUserID: reportStoreReporterUserID,
	}); err != nil {
		t.Fatalf("create comment report: %v", err)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM note_comments WHERE id = ?`, reportStoreCommentID); err != nil {
		t.Fatalf("delete comment: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, reportStoreNoteID); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	if got := countReports(t, ctx, db); got != 2 {
		t.Fatalf("report count after target deletion = %d, want 2 (no target foreign keys)", got)
	}
}

func TestReportStoreListInspectionRows(t *testing.T) {
	ctx := context.Background()
	store, db := newReportStoreTestStore(t, ctx)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreReporterUserID)
	insertAuthorStoreNote(t, ctx, db, reportStoreNoteID, reportStoreReporterUserID, 0)
	if _, err := db.ExecContext(ctx, `INSERT INTO note_comments (id, note_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)`, reportStoreCommentID, reportStoreNoteID, reportStoreReporterUserID, "comentário do alvo", 0); err != nil {
		t.Fatalf("insert comment: %v", err)
	}

	if _, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreNoteID, Reason: report.ReasonSpam, ReporterUserID: reportStoreReporterUserID,
	}); err != nil {
		t.Fatalf("create note report: %v", err)
	}
	details := "explicação"
	if _, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeComment, TargetID: reportStoreCommentID, Reason: report.ReasonHarassment, Details: &details, ReporterUserID: reportStoreReporterUserID,
	}); err != nil {
		t.Fatalf("create comment report: %v", err)
	}
	if _, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: "gone-note", Reason: report.ReasonOther, ReporterUserID: reportStoreReporterUserID,
	}); err != nil {
		t.Fatalf("create missing-target report: %v", err)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM note_comments WHERE id = ?`, reportStoreCommentID); err != nil {
		t.Fatalf("delete comment target: %v", err)
	}

	rows, err := store.ListInspectionRows(ctx)
	if err != nil {
		t.Fatalf("list inspection rows: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("inspection rows = %d, want 3", len(rows))
	}

	if rows[0].TargetType != report.TargetTypeNote || rows[0].TargetID != reportStoreNoteID || rows[0].TargetMissing {
		t.Fatalf("row 0 = %+v, want present note target", rows[0])
	}
	if rows[0].TargetSummary == nil || *rows[0].TargetSummary == "" {
		t.Fatalf("row 0 target summary = %v, want the note title", rows[0].TargetSummary)
	}
	if rows[1].TargetType != report.TargetTypeComment || rows[1].TargetID != reportStoreCommentID {
		t.Fatalf("row 1 = %+v, want comment target", rows[1])
	}
	if !rows[1].TargetMissing || rows[1].TargetSummary != nil {
		t.Fatalf("row 1 = %+v, want deleted comment (missing, nil summary)", rows[1])
	}
	if rows[1].Details == nil || *rows[1].Details != "explicação" {
		t.Fatalf("row 1 details = %v, want explicação", rows[1].Details)
	}
	if rows[2].TargetID != "gone-note" || !rows[2].TargetMissing {
		t.Fatalf("row 2 = %+v, want missing gone-note target", rows[2])
	}
}

func TestReportStoreListInspectionRowsShowsMissingAfterNoteStoreDelete(t *testing.T) {
	ctx := context.Background()
	store, db := newReportStoreTestStore(t, ctx)
	insertBareUsefulStoreUser(t, ctx, db, reportStoreReporterUserID)
	insertAuthorStoreNote(t, ctx, db, reportStoreNoteID, reportStoreReporterUserID, 0)

	if _, err := store.CreateReport(ctx, report.CreateInput{
		TargetType: report.TargetTypeNote, TargetID: reportStoreNoteID, Reason: report.ReasonSpam, ReporterUserID: reportStoreReporterUserID,
	}); err != nil {
		t.Fatalf("create note report: %v", err)
	}

	noteStore := newNoteStore(db, time.Now)
	if err := noteStore.DeleteNote(ctx, reportStoreNoteID, reportStoreReporterUserID); err != nil {
		t.Fatalf("delete note through store: %v", err)
	}

	rows, err := store.ListInspectionRows(ctx)
	if err != nil {
		t.Fatalf("list inspection rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("inspection rows = %d, want 1", len(rows))
	}
	if rows[0].TargetType != report.TargetTypeNote || rows[0].TargetID != reportStoreNoteID {
		t.Fatalf("row 0 = %+v, want the reported note", rows[0])
	}
	if !rows[0].TargetMissing || rows[0].TargetSummary != nil {
		t.Fatalf("row 0 = %+v, want missing note (TargetMissing true, nil summary)", rows[0])
	}
}

func TestCommentStoreFindCommentByID(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)
	created := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "comentário por id")

	found, err := store.FindCommentByID(ctx, string(created.ID))
	if err != nil {
		t.Fatalf("find comment by id: %v", err)
	}
	if diff := cmp.Diff(created, found); diff != "" {
		t.Fatalf("find by id mismatch (-want +got):\n%s", diff)
	}

	if _, err := store.FindCommentByID(ctx, "missing-comment-id"); !errors.Is(err, comment.ErrCommentNotFound) {
		t.Fatalf("missing comment error = %v, want %v", err, comment.ErrCommentNotFound)
	}
}

func newReportStoreTestStore(t *testing.T, ctx context.Context) (*ReportStore, *sql.DB) {
	t.Helper()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	return newReportStore(db, func() time.Time { return now }), db
}

func countReports(t *testing.T, ctx context.Context, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports`).Scan(&count); err != nil {
		t.Fatalf("count reports: %v", err)
	}
	return count
}

func rawReportRow(t *testing.T, ctx context.Context, db *sql.DB, reporter user.UserID, targetType, targetID string) (id string, createdAt int64, reason, details string, detailsValid bool) {
	t.Helper()
	var det sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT id, created_at, reason, details FROM reports WHERE reporter_user_id = ? AND target_type = ? AND target_id = ?`,
		string(reporter), targetType, targetID,
	).Scan(&id, &createdAt, &reason, &det); err != nil {
		t.Fatalf("read raw report: %v", err)
	}
	if det.Valid {
		detailsValid = true
		details = det.String
	}
	return id, createdAt, reason, details, detailsValid
}
