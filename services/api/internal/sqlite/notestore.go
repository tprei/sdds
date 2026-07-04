package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
)

const (
	insertNoteSQL = `
		INSERT INTO notes (id, title, body, category_slug, city_slug, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	listRecentNotesSQL = `
		SELECT id, title, body, category_slug, city_slug, created_at, updated_at
		FROM notes
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`
	findNoteSQL = `
		SELECT id, title, body, category_slug, city_slug, created_at, updated_at
		FROM notes
		WHERE id = ?
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

func (store *NoteStore) CreateNote(ctx context.Context, input note.CreateInput) (note.Note, error) {
	now := normalizeTime(store.clock())
	id, err := note.NewID()
	if err != nil {
		return note.Note{}, fmt.Errorf("create note id: %w", err)
	}

	created := note.Note{
		ID:           id,
		Title:        input.Title,
		Body:         input.Body,
		CategorySlug: input.CategorySlug,
		CitySlug:     input.CitySlug,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if _, err := store.db.ExecContext(
		ctx,
		insertNoteSQL,
		created.ID,
		created.Title,
		created.Body,
		string(created.CategorySlug),
		string(created.CitySlug),
		unixMillis(created.CreatedAt),
		unixMillis(created.UpdatedAt),
	); err != nil {
		return note.Note{}, fmt.Errorf("insert note: %w", err)
	}

	return created, nil
}

func (store *NoteStore) FindNote(ctx context.Context, id string) (note.Note, error) {
	found, err := scanNoteRow(store.db.QueryRowContext(ctx, findNoteSQL, id))
	if err == nil {
		return found, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return note.Note{}, note.ErrNoteNotFound
	}
	return note.Note{}, fmt.Errorf("find note: %w", err)
}

func (store *NoteStore) ListRecentNotes(ctx context.Context, limit int) (notes []note.Note, err error) {
	if limit < 1 {
		return nil, fmt.Errorf("list recent notes: limit must be positive")
	}

	rows, err := store.db.QueryContext(
		ctx,
		listRecentNotesSQL,
		limit,
	)
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

	return notes, nil
}

func scanNote(rows *sql.Rows) (note.Note, error) {
	return scanNoteValues(rows.Scan)
}

func scanNoteRow(row *sql.Row) (note.Note, error) {
	return scanNoteValues(row.Scan)
}

func scanNoteValues(scan func(dest ...any) error) (note.Note, error) {
	var found note.Note
	var categorySlug string
	var citySlug string
	var createdAt int64
	var updatedAt int64

	if err := scan(
		&found.ID,
		&found.Title,
		&found.Body,
		&categorySlug,
		&citySlug,
		&createdAt,
		&updatedAt,
	); err != nil {
		return note.Note{}, fmt.Errorf("scan note: %w", err)
	}

	found.CategorySlug = note.CategorySlug(categorySlug)
	found.CitySlug = note.CitySlug(citySlug)
	found.CreatedAt = timeFromUnixMillis(createdAt)
	found.UpdatedAt = timeFromUnixMillis(updatedAt)
	return found, nil
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
