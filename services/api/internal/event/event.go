package event

import (
	"time"

	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/report"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	SchemaVersion1                                     = 1
	SearchVersionFTS5V1                  SearchVersion = "fts5-v1"
	SearchVersionHybridSerafim100mFTS5V1 SearchVersion = "hybrid-serafim100m-fts5-v1"
)

// Kind identifies the user interaction or product surface that produced an event.
type Kind string

const (
	// Explore impressions fire when the authenticated user sees the Explore result set.
	KindExploreNotesImpression Kind = "explore_notes_impression"
	// Explore opens fire when the user opens a note from Explore.
	KindExploreNoteOpened Kind = "explore_note_opened"
	// Search submissions fire when the user commits a non-empty search.
	KindSearchSubmitted Kind = "search_submitted"
	// Search result impressions fire when a committed search result set is rendered.
	KindSearchResultsImpression Kind = "search_results_impression"
	// Search result opens fire when the user opens a result from that search.
	KindSearchResultOpened Kind = "search_result_opened"
	// Reformulations fire when the user submits a new search after a prior search.
	KindSearchReformulated Kind = "search_reformulated"
	// No-result events fire when a committed search returns no notes.
	KindSearchNoResults Kind = "search_no_results"
	// Useful events fire after the user successfully marks a note useful.
	KindNoteMarkedUseful Kind = "note_marked_useful"
	// Unuseful events fire after the user successfully removes a useful mark.
	KindNoteUnmarkedUseful Kind = "note_unmarked_useful"
	// Comment events fire after the user's comment is successfully created.
	KindCommentCreated Kind = "comment_created"
	// Report events fire after the user's report is successfully created.
	KindReportCreated Kind = "report_created"
	// Publication events fire after the user's note is successfully published.
	KindNotePublished Kind = "note_published"
)

type SearchVersion string

type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformWeb     Platform = "web"
)

// RetrievalSource describes the retrieval path that won for a result. A hybrid
// value intentionally records only the merged result's final classification;
// per-source scores and pre-merge prioritization are not part of the v1 event
// contract.
type RetrievalSource string

const (
	RetrievalSourceLexical  RetrievalSource = "lexical"
	RetrievalSourceSemantic RetrievalSource = "semantic"
	RetrievalSourceHybrid   RetrievalSource = "hybrid"
)

// Input is the untrusted client envelope before normalization.
//
// Record is the validated internal form persisted by the event store. Keeping
// both shapes here lets every event kind share one validation boundary without
// allowing transport-specific fields into persistence.
type Input struct {
	ID             string
	Kind           Kind
	OccurredAt     int64
	UserID         user.UserID
	InstallationID *string
	Platform       Platform
	AppVersion     *string
	SchemaVersion  int
	Payload        Payload
}

type Record struct {
	ID             string
	Kind           Kind
	OccurredAt     time.Time
	UserID         user.UserID
	InstallationID *string
	Platform       Platform
	AppVersion     *string
	SchemaVersion  int
	Payload        Payload
}

type ValidationProblem struct {
	Field string
	Code  string
}

type AppendBatchResult struct {
	AcceptedCount  int
	DuplicateCount int
}

type Payload interface {
	payload()
}

type UsefulContext interface {
	usefulContext()
}

type ExploreResult struct {
	NoteID string `json:"note_id"`
	Rank   int    `json:"rank"`
}

type SearchResult struct {
	NoteID          string          `json:"note_id"`
	Rank            int             `json:"rank"`
	RetrievalSource RetrievalSource `json:"retrieval_source"`
}

type ExploreNotesImpressionPayload struct {
	CategorySlug *note.CategorySlug `json:"category_slug"`
	ResultCount  int                `json:"result_count"`
	Results      []ExploreResult    `json:"results"`
}

func (ExploreNotesImpressionPayload) payload() {}

type ExploreNoteOpenedPayload struct {
	NoteID       string             `json:"note_id"`
	Rank         int                `json:"rank"`
	CategorySlug *note.CategorySlug `json:"category_slug"`
}

func (ExploreNoteOpenedPayload) payload() {}

type SearchSubmittedPayload struct {
	SearchID      string             `json:"search_id"`
	SearchVersion SearchVersion      `json:"search_version"`
	Query         string             `json:"query"`
	CategorySlug  *note.CategorySlug `json:"category_slug"`
}

func (SearchSubmittedPayload) payload() {}

type SearchResultsImpressionPayload struct {
	SearchID      string             `json:"search_id"`
	SearchVersion SearchVersion      `json:"search_version"`
	Query         string             `json:"query"`
	CategorySlug  *note.CategorySlug `json:"category_slug"`
	ResultCount   int                `json:"result_count"`
	Results       []SearchResult     `json:"results"`
}

func (SearchResultsImpressionPayload) payload() {}

type SearchResultOpenedPayload struct {
	SearchID        string          `json:"search_id"`
	SearchVersion   SearchVersion   `json:"search_version"`
	NoteID          string          `json:"note_id"`
	Rank            int             `json:"rank"`
	RetrievalSource RetrievalSource `json:"retrieval_source"`
}

func (SearchResultOpenedPayload) payload() {}

type SearchReformulatedPayload struct {
	PreviousSearchID      string             `json:"previous_search_id"`
	PreviousSearchVersion SearchVersion      `json:"previous_search_version"`
	SearchID              string             `json:"search_id"`
	SearchVersion         SearchVersion      `json:"search_version"`
	PreviousQuery         string             `json:"previous_query"`
	Query                 string             `json:"query"`
	PreviousCategorySlug  *note.CategorySlug `json:"previous_category_slug"`
	CategorySlug          *note.CategorySlug `json:"category_slug"`
}

func (SearchReformulatedPayload) payload() {}

type SearchNoResultsPayload struct {
	SearchID      string             `json:"search_id"`
	SearchVersion SearchVersion      `json:"search_version"`
	Query         string             `json:"query"`
	CategorySlug  *note.CategorySlug `json:"category_slug"`
	ResultCount   int                `json:"result_count"`
}

func (SearchNoResultsPayload) payload() {}

type SearchUsefulContext struct {
	Source          string          `json:"source"`
	SearchID        string          `json:"search_id"`
	SearchVersion   SearchVersion   `json:"search_version"`
	Rank            int             `json:"rank"`
	RetrievalSource RetrievalSource `json:"retrieval_source"`
}

func (SearchUsefulContext) usefulContext() {}

type ExploreUsefulContext struct {
	Source       string             `json:"source"`
	Rank         int                `json:"rank"`
	CategorySlug *note.CategorySlug `json:"category_slug"`
}

func (ExploreUsefulContext) usefulContext() {}

type NoteDetailUsefulContext struct {
	Source string `json:"source"`
}

func (NoteDetailUsefulContext) usefulContext() {}

type AuthorProfileUsefulContext struct {
	Source string `json:"source"`
}

func (AuthorProfileUsefulContext) usefulContext() {}

type NoteMarkedUsefulPayload struct {
	NoteID  string        `json:"note_id"`
	Context UsefulContext `json:"context"`
}

func (NoteMarkedUsefulPayload) payload() {}

type NoteUnmarkedUsefulPayload struct {
	NoteID  string        `json:"note_id"`
	Context UsefulContext `json:"context"`
}

func (NoteUnmarkedUsefulPayload) payload() {}

// CommentCreatedPayload records a comment creation. ParentCommentID is nil
// for a top-level comment and the validated parent UUID for a reply.
type CommentCreatedPayload struct {
	NoteID          string             `json:"note_id"`
	CommentID       comment.CommentID  `json:"comment_id"`
	ParentCommentID *comment.CommentID `json:"parent_comment_id"`
}

func (CommentCreatedPayload) payload() {}

type ReportCreatedPayload struct {
	ReportID   report.ID         `json:"report_id"`
	TargetType report.TargetType `json:"target_type"`
	TargetID   string            `json:"target_id"`
}

func (ReportCreatedPayload) payload() {}

type NotePublishedPayload struct {
	NoteID       string            `json:"note_id"`
	CategorySlug note.CategorySlug `json:"category_slug"`
}

func (NotePublishedPayload) payload() {}
