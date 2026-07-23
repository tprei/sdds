package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
	modernsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	insertNoteSQL = `
		INSERT INTO notes (id, user_id, title, body, category_slug, place_slug, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	insertNoteSearchSQL = `
		INSERT INTO note_search (note_id, title, body)
		VALUES (?, ?, ?)
	`
	findNoteCreateRequestSQL = `
		SELECT request_sha256, note_id
		FROM note_create_requests
		WHERE user_id = ? AND client_request_id = ?
	`
	insertNoteCreateRequestSQL = `
		INSERT INTO note_create_requests (user_id, client_request_id, request_sha256, note_id, created_at)
		VALUES (?, ?, ?, ?, ?)
	`
)

type noteCreateRequestQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type noteCreateReceiptRace struct{ insertErr error }

func (race noteCreateReceiptRace) Error() string { return race.insertErr.Error() }

func (store *NoteStore) CreateNote(ctx context.Context, input note.CreateInput) (created note.Note, err error) {
	normalized := note.NormalizeCreateInput(input)
	if problems := note.ValidateCreateInput(normalized); len(problems) > 0 {
		return note.Note{}, fmt.Errorf("create note: invalid input")
	}
	requestSHA256 := note.CreateRequestFingerprint(normalized)

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return note.Note{}, fmt.Errorf("begin create note: %w", err)
	}
	now := normalizeTime(store.clock())
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			rollbackErr = fmt.Errorf("rollback create note: %w", rollbackErr)
			if err == nil {
				err = rollbackErr
			} else {
				err = errors.Join(err, rollbackErr)
			}
		}
	}()

	created, err = store.createNoteInTransaction(ctx, tx, normalized, requestSHA256, now)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return note.Note{}, fmt.Errorf("commit create note: %w", err)
		}
		return created, nil
	}

	var receiptRace noteCreateReceiptRace
	if !errors.As(err, &receiptRace) {
		return note.Note{}, err
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return note.Note{}, errors.Join(
			fmt.Errorf("rollback conflicting note create: %w", rollbackErr),
			fmt.Errorf("insert note create request: %w", receiptRace.insertErr),
		)
	}
	return store.reconcileNoteCreateReceipt(ctx, normalized, requestSHA256, receiptRace.insertErr)
}

func (store *NoteStore) createNoteInTransaction(ctx context.Context, tx *sql.Tx, input note.CreateInput, requestSHA256 string, now time.Time) (note.Note, error) {
	storedSHA256, storedNoteID, found, err := readNoteCreateRequest(ctx, tx, string(input.UserID), input.ClientRequestID)
	if err != nil {
		return note.Note{}, fmt.Errorf("read note create request: %w", err)
	}
	if found {
		if storedSHA256 != requestSHA256 {
			return note.Note{}, note.ErrIdempotencyConflict
		}
		created, err := loadNoteWithOrderedImages(ctx, tx, tx, storedNoteID, input.UserID)
		if err != nil {
			return note.Note{}, fmt.Errorf("load replayed note: %w", err)
		}
		return created, nil
	}
	if err := validateNoteCreateCatalog(ctx, tx, input); err != nil {
		return note.Note{}, err
	}

	id, err := note.NewID()
	if err != nil {
		return note.Note{}, fmt.Errorf("create note id: %w", err)
	}
	created := note.Note{
		ID:           id,
		UserID:       input.UserID,
		Title:        input.Title,
		Body:         input.Body,
		CategorySlug: input.CategorySlug,
		PlaceSlug:    input.PlaceSlug,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := insertNoteAndSearch(ctx, tx, created); err != nil {
		return note.Note{}, err
	}
	if err := associateImageUploads(ctx, tx, input, created.ID, now); err != nil {
		return note.Note{}, err
	}
	if err := insertNoteCreateRequest(ctx, tx, input, requestSHA256, created.ID, now); err != nil {
		return note.Note{}, err
	}
	created, err = loadNoteWithOrderedImages(ctx, tx, tx, created.ID, input.UserID)
	if err != nil {
		return note.Note{}, fmt.Errorf("load created note: %w", err)
	}
	return created, nil
}

func validateNoteCreateCatalog(ctx context.Context, tx *sql.Tx, input note.CreateInput) error {
	problems := make([]note.ValidationProblem, 0, 2)
	if _, err := scanCategoryRow(tx.QueryRowContext(ctx, findActiveCategorySQL, string(input.CategorySlug))); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			problems = append(problems, note.ValidationProblem{Field: "category_slug", Message: "unknown"})
		} else {
			return fmt.Errorf("check active category: %w", err)
		}
	}
	if input.PlaceSlug != "" {
		if _, err := scanPlaceRow(tx.QueryRowContext(ctx, findActivePlaceSQL, string(input.PlaceSlug))); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				problems = append(problems, note.ValidationProblem{Field: "place_slug", Message: "unknown"})
			} else {
				return fmt.Errorf("check active place: %w", err)
			}
		}
	}
	if len(problems) > 0 {
		return &note.CatalogValidationError{Problems: problems}
	}
	return nil
}

func insertNoteAndSearch(ctx context.Context, tx *sql.Tx, created note.Note) error {
	if _, err := tx.ExecContext(
		ctx,
		insertNoteSQL,
		created.ID,
		created.UserID,
		created.Title,
		created.Body,
		string(created.CategorySlug),
		nullablePlaceSlug(created.PlaceSlug),
		unixMillis(created.CreatedAt),
		unixMillis(created.UpdatedAt),
	); err != nil {
		return fmt.Errorf("insert note: %w", err)
	}
	if _, err := tx.ExecContext(ctx, insertNoteSearchSQL, created.ID, created.Title, created.Body); err != nil {
		return fmt.Errorf("insert note search: %w", err)
	}
	return nil
}

func insertNoteCreateRequest(ctx context.Context, tx *sql.Tx, input note.CreateInput, requestSHA256, noteID string, now time.Time) error {
	if _, err := tx.ExecContext(
		ctx,
		insertNoteCreateRequestSQL,
		input.UserID,
		input.ClientRequestID,
		requestSHA256,
		noteID,
		unixMillis(now),
	); err != nil {
		if isNoteCreateReceiptConstraintError(err) {
			return noteCreateReceiptRace{insertErr: err}
		}
		return fmt.Errorf("insert note create request: %w", err)
	}
	return nil
}

func isNoteCreateReceiptConstraintError(err error) bool {
	var sqliteErr *modernsqlite.Error
	return errors.As(err, &sqliteErr) &&
		(sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY ||
			sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE)
}

func (store *NoteStore) reconcileNoteCreateReceipt(ctx context.Context, input note.CreateInput, requestSHA256 string, insertErr error) (note.Note, error) {
	storedSHA256, storedNoteID, found, err := readNoteCreateRequest(ctx, store.db, string(input.UserID), input.ClientRequestID)
	if err != nil {
		return note.Note{}, fmt.Errorf("read conflicting note create request: %w", err)
	}
	if !found {
		return note.Note{}, fmt.Errorf("insert note create request: %w", insertErr)
	}
	if storedSHA256 != requestSHA256 {
		return note.Note{}, note.ErrIdempotencyConflict
	}
	created, err := loadNoteWithOrderedImages(ctx, store.db, store.db, storedNoteID, input.UserID)
	if err != nil {
		return note.Note{}, fmt.Errorf("load conflicting note: %w", err)
	}
	return created, nil
}

func readNoteCreateRequest(ctx context.Context, queryer noteCreateRequestQuerier, userID, clientRequestID string) (requestSHA256, noteID string, found bool, err error) {
	err = queryer.QueryRowContext(ctx, findNoteCreateRequestSQL, userID, clientRequestID).Scan(&requestSHA256, &noteID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return requestSHA256, noteID, true, nil
}
