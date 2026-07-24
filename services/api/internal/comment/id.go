package comment

import (
	"fmt"

	"github.com/google/uuid"
)

func NewID() (CommentID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("create uuid v7: %w", err)
	}
	return CommentID(id.String()), nil
}
