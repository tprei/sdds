package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tprei/sdds/services/api/internal/report"
	"github.com/tprei/sdds/services/api/internal/user"
)

var _ report.Store = (*ReportStore)(nil)

type ReportStore struct {
	db    *sql.DB
	clock func() time.Time
}

func NewReportStore(db *sql.DB) *ReportStore {
	return newReportStore(db, time.Now)
}

func newReportStore(db *sql.DB, clock func() time.Time) *ReportStore {
	return &ReportStore{db: db, clock: clock}
}

const (
	insertReportSQL = `
		INSERT INTO reports (id, reporter_user_id, target_type, target_id, reason, details, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (reporter_user_id, target_type, target_id) DO NOTHING
	`
	overwriteReportSQL = `
		UPDATE reports SET reason = ?, details = ?
		WHERE reporter_user_id = ? AND target_type = ? AND target_id = ?
	`
	findReportByTargetSQL = `
		SELECT id, reporter_user_id, target_type, target_id, reason, details, created_at
		FROM reports
		WHERE reporter_user_id = ? AND target_type = ? AND target_id = ?
	`
	listReportsForInspectionSQL = `
		SELECT
			r.report_page_key,
			r.id,
			r.created_at,
			r.reporter_user_id,
			r.target_type,
			r.target_id,
			r.reason,
			r.details,
			CASE
				WHEN r.target_type = 'note' THEN n.title
				WHEN r.target_type = 'comment' THEN substr(c.body, 1, 240)
			END AS target_summary,
			CASE
				WHEN r.target_type = 'note' AND n.id IS NULL THEN 1
				WHEN r.target_type = 'comment' AND c.id IS NULL THEN 1
				ELSE 0
			END AS target_missing
		FROM reports AS r
		LEFT JOIN notes AS n
			ON r.target_type = 'note' AND n.id = r.target_id
		LEFT JOIN note_comments AS c
			ON r.target_type = 'comment' AND c.id = r.target_id
		ORDER BY r.report_page_key ASC
	`
)

func (store *ReportStore) CreateReport(ctx context.Context, input report.CreateInput) (report.CreateResult, error) {
	normalized := report.NormalizeCreateInput(input)
	if problems := report.ValidateCreateInput(normalized); len(problems) > 0 {
		return report.CreateResult{}, fmt.Errorf("create report: invalid input")
	}
	if err := report.ValidateInternal(normalized); err != nil {
		return report.CreateResult{}, fmt.Errorf("create report: %w", err)
	}

	id, err := report.NewID()
	if err != nil {
		return report.CreateResult{}, fmt.Errorf("create report id: %w", err)
	}
	now := normalizeTime(store.clock())

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return report.CreateResult{}, fmt.Errorf("begin create report: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(
		ctx,
		insertReportSQL,
		string(id),
		string(normalized.ReporterUserID),
		string(normalized.TargetType),
		normalized.TargetID,
		string(normalized.Reason),
		reportDetailsValue(normalized.Details),
		unixMillis(now),
	)
	if err != nil {
		return report.CreateResult{}, fmt.Errorf("insert report: %w", err)
	}

	created := false
	affected, err := result.RowsAffected()
	if err != nil {
		return report.CreateResult{}, fmt.Errorf("read inserted report count: %w", err)
	}
	if affected == 1 {
		created = true
	} else if _, err := tx.ExecContext(
		ctx,
		overwriteReportSQL,
		string(normalized.Reason),
		reportDetailsValue(normalized.Details),
		string(normalized.ReporterUserID),
		string(normalized.TargetType),
		normalized.TargetID,
	); err != nil {
		return report.CreateResult{}, fmt.Errorf("overwrite report: %w", err)
	}

	found, err := scanReportRow(tx.QueryRowContext(
		ctx,
		findReportByTargetSQL,
		string(normalized.ReporterUserID),
		string(normalized.TargetType),
		normalized.TargetID,
	))
	if err != nil {
		return report.CreateResult{}, fmt.Errorf("load report: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return report.CreateResult{}, fmt.Errorf("commit create report: %w", err)
	}

	return report.CreateResult{Report: found, Created: created}, nil
}

// ListInspectionRows returns every report joined to its target content for the
// read-only operator inspection view, ordered by insertion sequence. A deleted
// target yields a nil TargetSummary and TargetMissing true.
func (store *ReportStore) ListInspectionRows(ctx context.Context) ([]report.InspectionRow, error) {
	rows, err := store.db.QueryContext(ctx, listReportsForInspectionSQL)
	if err != nil {
		return nil, fmt.Errorf("query report inspection rows: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	results := make([]report.InspectionRow, 0)
	for rows.Next() {
		var (
			row           report.InspectionRow
			targetType    string
			reason        string
			details       sql.NullString
			targetSummary sql.NullString
			targetMissing int
		)
		if err := rows.Scan(
			&row.ReportPageKey,
			&row.ID,
			&row.CreatedAt,
			&row.ReporterUserID,
			&targetType,
			&row.TargetID,
			&reason,
			&details,
			&targetSummary,
			&targetMissing,
		); err != nil {
			return nil, fmt.Errorf("scan report inspection row: %w", err)
		}
		row.TargetType = report.TargetType(targetType)
		row.Reason = report.Reason(reason)
		if details.Valid {
			value := details.String
			row.Details = &value
		}
		if targetSummary.Valid {
			value := targetSummary.String
			row.TargetSummary = &value
		}
		row.TargetMissing = targetMissing == 1
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read report inspection rows: %w", err)
	}
	return results, nil
}

func scanReportRow(row *sql.Row) (report.Report, error) {
	var found report.Report
	var id string
	var reporterUserID string
	var targetType string
	var targetID string
	var reason string
	var details sql.NullString
	var createdAt int64
	if err := row.Scan(
		&id,
		&reporterUserID,
		&targetType,
		&targetID,
		&reason,
		&details,
		&createdAt,
	); err != nil {
		return report.Report{}, fmt.Errorf("scan report: %w", err)
	}

	found.ID = report.ID(id)
	found.ReporterUserID = user.UserID(reporterUserID)
	found.TargetType = report.TargetType(targetType)
	found.TargetID = targetID
	found.Reason = report.Reason(reason)
	if details.Valid {
		value := details.String
		found.Details = &value
	}
	found.CreatedAt = timeFromUnixMillis(createdAt)
	return found, nil
}

func reportDetailsValue(details *string) any {
	if details == nil {
		return nil
	}
	return *details
}
