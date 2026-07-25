package report

import (
	"context"
	"time"

	"github.com/tprei/sdds/services/api/internal/user"
)

const DetailsMaxLength = 1000

type ID string

type TargetType string

const (
	TargetTypeNote    TargetType = "note"
	TargetTypeComment TargetType = "comment"
)

type Reason string

const (
	ReasonSpam                Reason = "spam"
	ReasonHarassment          Reason = "harassment"
	ReasonHarmfulOrMisleading Reason = "harmful_or_misleading"
	ReasonOther               Reason = "other"
)

type Report struct {
	ID             ID
	TargetType     TargetType
	TargetID       string
	Reason         Reason
	Details        *string
	ReporterUserID user.UserID
	CreatedAt      time.Time
}

type CreateInput struct {
	TargetType     TargetType
	TargetID       string
	Reason         Reason
	Details        *string
	ReporterUserID user.UserID
}

type CreateResult struct {
	Report  Report
	Created bool
}

type Store interface {
	CreateReport(context.Context, CreateInput) (CreateResult, error)
}
