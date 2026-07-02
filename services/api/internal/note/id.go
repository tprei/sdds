package note

import (
	"fmt"

	"github.com/google/uuid"
)

func NewID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("create uuid v7: %w", err)
	}
	return id.String(), nil
}
