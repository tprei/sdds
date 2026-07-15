package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/user"
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
	consumeImageUploadSQL = `
		UPDATE image_uploads
		SET state = 'consumed', consumed_note_id = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND state = 'ready'
			AND consumed_note_id IS NULL AND expires_at > ?
	`
	findImageUploadForAssociationSQL = `
		SELECT id, user_id, storage_key, state, consumed_note_id,
			content_type, byte_size, width, height, sha256,
			created_at, updated_at, expires_at
		FROM image_uploads
		WHERE id = ?
	`
	insertNoteImageSQL = `
		INSERT INTO note_images (
			id, note_id, storage_key, content_type, byte_size, width, height,
			sha256, position, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	listRecentNotesSQL = `
		SELECT
			notes.id,
			notes.user_id,
			notes.title,
			notes.body,
			notes.category_slug,
			notes.place_slug,
			authors.id,
			authors.display_name,
			notes.created_at,
			notes.updated_at
		FROM notes
		JOIN authors ON authors.user_id = notes.user_id
		ORDER BY notes.created_at DESC, notes.id DESC
		LIMIT ?
	`
	listRecentNotesByCategorySQL = `
		SELECT
			notes.id,
			notes.user_id,
			notes.title,
			notes.body,
			notes.category_slug,
			notes.place_slug,
			authors.id,
			authors.display_name,
			notes.created_at,
			notes.updated_at
		FROM notes
		JOIN authors ON authors.user_id = notes.user_id
		WHERE notes.category_slug = ?
		ORDER BY notes.created_at DESC, notes.id DESC
		LIMIT ?
	`
	findNoteSQL = `
		SELECT
			notes.id,
			notes.user_id,
			notes.title,
			notes.body,
			notes.category_slug,
			notes.place_slug,
			authors.id,
			authors.display_name,
			notes.created_at,
			notes.updated_at
		FROM notes
		JOIN authors ON authors.user_id = notes.user_id
		WHERE notes.id = ?
	`
	searchNotesOrderSQL = `
		ORDER BY bm25(note_search, 0.0, 6.0, 1.0), notes.created_at DESC, notes.id DESC
	`
	searchNotesSQL = `
		SELECT
			notes.id,
			notes.user_id,
			notes.title,
			notes.body,
			notes.category_slug,
			notes.place_slug,
			authors.id,
			authors.display_name,
			notes.created_at,
			notes.updated_at
		FROM note_search
		JOIN notes ON notes.id = note_search.note_id
		JOIN authors ON authors.user_id = notes.user_id
		WHERE note_search MATCH ?
	` + searchNotesOrderSQL + `
		LIMIT ?
	`
	searchNotesByCategorySQL = `
		SELECT
			notes.id,
			notes.user_id,
			notes.title,
			notes.body,
			notes.category_slug,
			notes.place_slug,
			authors.id,
			authors.display_name,
			notes.created_at,
			notes.updated_at
		FROM note_search
		JOIN notes ON notes.id = note_search.note_id
		JOIN authors ON authors.user_id = notes.user_id
		WHERE note_search MATCH ?
			AND notes.category_slug = ?
	` + searchNotesOrderSQL + `
		LIMIT ?
	`
)

var _ note.Store = (*NoteStore)(nil)

type NoteStore struct {
	db    *sql.DB
	clock func() time.Time
}

func NewNoteStore(db *sql.DB) *NoteStore {
	return newNoteStore(db, time.Now)
}

func newNoteStore(db *sql.DB, clock func() time.Time) *NoteStore {
	return &NoteStore{db: db, clock: clock}
}

func (store *NoteStore) CreateNote(ctx context.Context, input note.CreateInput) (created note.Note, err error) {
	normalized := note.NormalizeCreateInput(input)
	if problems := note.ValidateCreateInput(normalized); len(problems) > 0 {
		return note.Note{}, fmt.Errorf("create note: invalid input")
	}
	requestSHA256 := noteCreateFingerprint(normalized)

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return note.Note{}, fmt.Errorf("begin create note: %w", err)
	}
	now := normalizeTime(store.clock())
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback create note: %w", rollbackErr)
		}
	}()

	storedSHA256, storedNoteID, found, err := readNoteCreateRequest(ctx, tx, string(normalized.UserID), normalized.ClientRequestID)
	if err != nil {
		return note.Note{}, fmt.Errorf("read note create request: %w", err)
	}
	if found {
		if storedSHA256 != requestSHA256 {
			return note.Note{}, note.ErrIdempotencyConflict
		}
		created, err = store.loadNoteForCreate(ctx, tx, storedNoteID)
		if err != nil {
			return note.Note{}, fmt.Errorf("load replayed note: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return note.Note{}, fmt.Errorf("commit replayed note: %w", err)
		}
		return created, nil
	}
	if _, err := scanCategoryRow(tx.QueryRowContext(ctx, findActiveCategorySQL, string(normalized.CategorySlug))); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return note.Note{}, fmt.Errorf("constraint failed: %w", note.ErrCategoryNotFound)
		}
		return note.Note{}, fmt.Errorf("check active category: %w", err)
	}
	if normalized.PlaceSlug != "" {
		if _, err := scanPlaceRow(tx.QueryRowContext(ctx, findActivePlaceSQL, string(normalized.PlaceSlug))); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return note.Note{}, fmt.Errorf("constraint failed: %w", note.ErrPlaceNotFound)
			}
			return note.Note{}, fmt.Errorf("check active place: %w", err)
		}
	}

	id, err := note.NewID()
	if err != nil {
		return note.Note{}, fmt.Errorf("create note id: %w", err)
	}
	created = note.Note{
		ID:           id,
		UserID:       normalized.UserID,
		Title:        normalized.Title,
		Body:         normalized.Body,
		CategorySlug: normalized.CategorySlug,
		PlaceSlug:    normalized.PlaceSlug,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

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
		return note.Note{}, fmt.Errorf("insert note: %w", err)
	}
	if _, err := tx.ExecContext(ctx, insertNoteSearchSQL, created.ID, created.Title, created.Body); err != nil {
		return note.Note{}, fmt.Errorf("insert note search: %w", err)
	}

	for position, imageUploadID := range normalized.ImageUploadIDs {
		upload, err := consumeImageUpload(ctx, tx, string(normalized.UserID), imageUploadID, created.ID, now)
		if err != nil {
			return note.Note{}, fmt.Errorf("consume image upload %q: %w", imageUploadID, err)
		}
		upload.Position = position
		if _, err := tx.ExecContext(
			ctx,
			insertNoteImageSQL,
			upload.ID,
			created.ID,
			upload.StorageKey,
			upload.ContentType,
			upload.ByteSize,
			upload.Width,
			upload.Height,
			upload.SHA256,
			upload.Position,
			unixMillis(now),
			unixMillis(now),
		); err != nil {
			return note.Note{}, fmt.Errorf("insert note image: %w", err)
		}
	}

	if _, insertErr := tx.ExecContext(
		ctx,
		insertNoteCreateRequestSQL,
		normalized.UserID,
		normalized.ClientRequestID,
		requestSHA256,
		created.ID,
		unixMillis(now),
	); insertErr != nil {
		if !isUniqueConstraintError(insertErr) {
			return note.Note{}, fmt.Errorf("insert note create request: %w", insertErr)
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return note.Note{}, errors.Join(
				fmt.Errorf("rollback conflicting note create: %w", rollbackErr),
				fmt.Errorf("insert note create request: %w", insertErr),
			)
		}
		storedSHA256, storedNoteID, found, readErr := readNoteCreateRequest(ctx, store.db, string(normalized.UserID), normalized.ClientRequestID)
		if readErr != nil {
			return note.Note{}, fmt.Errorf("read conflicting note create request: %w", readErr)
		}
		if !found {
			return note.Note{}, fmt.Errorf("insert note create request: %w", insertErr)
		}
		if storedSHA256 != requestSHA256 {
			return note.Note{}, note.ErrIdempotencyConflict
		}
		created, err = store.loadNoteForCreate(ctx, store.db, storedNoteID)
		if err != nil {
			return note.Note{}, fmt.Errorf("load conflicting note: %w", err)
		}
		return created, nil
	}

	created, err = store.loadNoteForCreate(ctx, tx, created.ID)
	if err != nil {
		return note.Note{}, fmt.Errorf("load created note: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return note.Note{}, fmt.Errorf("commit create note: %w", err)
	}
	return created, nil
}

func (store *NoteStore) FindNote(ctx context.Context, id string) (note.Note, error) {
	found, err := scanNoteRow(store.db.QueryRowContext(ctx, findNoteSQL, id))
	if err == nil {
		notes := []note.Note{found}
		if err := store.hydrateNoteImages(ctx, notes); err != nil {
			return note.Note{}, fmt.Errorf("hydrate note images: %w", err)
		}
		return notes[0], nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return note.Note{}, note.ErrNoteNotFound
	}
	return note.Note{}, fmt.Errorf("find note: %w", err)
}

func (store *NoteStore) ListRecentNotes(ctx context.Context, input note.ListInput) (notes []note.Note, err error) {
	normalized := note.NormalizeListInput(input)
	if problems := note.ValidateListInput(normalized); len(problems) > 0 {
		return nil, fmt.Errorf("list recent notes: invalid input")
	}

	query := listRecentNotesSQL
	args := []any{normalized.Limit}
	if normalized.CategorySlug != "" {
		query = listRecentNotesByCategorySQL
		args = []any{string(normalized.CategorySlug), normalized.Limit}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query recent notes: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close recent notes rows: %w", closeErr)
		}
	}()

	notes = make([]note.Note, 0)
	for rows.Next() {
		found, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, found)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read recent notes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close recent notes rows: %w", err)
	}
	if err := store.hydrateNoteImages(ctx, notes); err != nil {
		return nil, fmt.Errorf("hydrate recent note images: %w", err)
	}

	return notes, nil
}

func (store *NoteStore) SearchNotes(ctx context.Context, input note.SearchInput) (notes []note.Note, err error) {
	normalized := note.NormalizeSearchInput(input)
	if problems := note.ValidateSearchInput(normalized); len(problems) > 0 {
		return nil, fmt.Errorf("search notes: invalid input")
	}

	matchExpression := noteSearchMatchExpression(normalized.Query)
	if matchExpression == "" {
		return []note.Note{}, nil
	}

	query := searchNotesSQL
	args := []any{matchExpression, normalized.Limit}
	if normalized.CategorySlug != "" {
		query = searchNotesByCategorySQL
		args = []any{matchExpression, string(normalized.CategorySlug), normalized.Limit}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query note search: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close search notes rows: %w", closeErr)
		}
	}()

	notes = make([]note.Note, 0)
	for rows.Next() {
		found, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, found)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read search notes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close search notes rows: %w", err)
	}
	if err := store.hydrateNoteImages(ctx, notes); err != nil {
		return nil, fmt.Errorf("hydrate search note images: %w", err)
	}

	return notes, nil
}

func noteSearchMatchExpression(query string) string {
	tokens := noteSearchTokens(query)
	quoted := make([]string, 0, len(tokens))
	for _, token := range tokens {
		quoted = append(quoted, quoteNoteSearchToken(token))
	}
	return strings.Join(quoted, " AND ")
}

func noteSearchTokens(query string) []string {
	tokens := make([]string, 0)
	var current strings.Builder
	for _, value := range query {
		if unicode.IsLetter(value) || unicode.IsDigit(value) {
			current.WriteRune(value)
			continue
		}
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func quoteNoteSearchToken(token string) string {
	return `"` + strings.ReplaceAll(token, `"`, `""`) + `"`
}

func scanNote(rows *sql.Rows) (note.Note, error) {
	return scanNoteValues(rows.Scan)
}

func scanNoteRow(row *sql.Row) (note.Note, error) {
	return scanNoteValues(row.Scan)
}

func scanNoteValues(scan func(dest ...any) error) (note.Note, error) {
	var found note.Note
	found.Images = make([]note.Image, 0)
	var userID string
	var categorySlug string
	var placeSlug sql.NullString
	var authorID string
	var createdAt int64
	var updatedAt int64

	if err := scan(
		&found.ID,
		&userID,
		&found.Title,
		&found.Body,
		&categorySlug,
		&placeSlug,
		&authorID,
		&found.Author.DisplayName,
		&createdAt,
		&updatedAt,
	); err != nil {
		return note.Note{}, fmt.Errorf("scan note: %w", err)
	}

	found.UserID = user.UserID(userID)
	found.CategorySlug = note.CategorySlug(categorySlug)
	if placeSlug.Valid {
		found.PlaceSlug = note.PlaceSlug(placeSlug.String)
	}
	found.Author.ID = author.AuthorID(authorID)
	found.CreatedAt = timeFromUnixMillis(createdAt)
	found.UpdatedAt = timeFromUnixMillis(updatedAt)
	return found, nil
}

func nullablePlaceSlug(slug note.PlaceSlug) any {
	if slug == "" {
		return nil
	}
	return string(slug)
}

func normalizeTime(value time.Time) time.Time {
	return timeFromUnixMillis(unixMillis(value))
}

func unixMillis(value time.Time) int64 {
	return value.UTC().UnixMilli()
}

func timeFromUnixMillis(value int64) time.Time {
	return time.UnixMilli(value).UTC()
}
