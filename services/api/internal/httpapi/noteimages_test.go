package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

func TestGetNoteMapsOrderedImageMetadataToResponse(t *testing.T) {
	createdAt := time.UnixMilli(1782993600000).UTC()
	updatedAt := time.UnixMilli(1782993601000).UTC()
	router := newTestRouter(fakeNoteStore{
		findNote: func(_ context.Context, id string, _ user.UserID) (note.Note, error) {
			return note.Note{
				ID:           id,
				Title:        "Café bom",
				Body:         "Tem pão de queijo decente.",
				CategorySlug: note.CategorySlugFood,
				Images: []note.Image{
					{
						ID:          "image-zero",
						ContentType: "image/jpeg",
						ByteSize:    481234,
						Width:       1200,
						Height:      900,
						Position:    0,
						CreatedAt:   createdAt,
						UpdatedAt:   updatedAt,
					},
				},
			}, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes/image-note", nil)
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body openapi.Note
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := []openapi.NoteImage{
		{
			Id:          "image-zero",
			Url:         "/v1/media/images/image-zero",
			ContentType: openapi.NoteImageContentTypeImagejpeg,
			ByteSize:    481234,
			Width:       1200,
			Height:      900,
			Position:    0,
			CreatedAt:   createdAt.UnixMilli(),
			UpdatedAt:   updatedAt.UnixMilli(),
		},
	}
	if diff := cmp.Diff(want, body.Images); diff != "" {
		t.Fatalf("image response mismatch (-want +got):\n%s", diff)
	}
	if len(body.Images) != 1 || body.Images[0].Url[0] != '/' {
		t.Fatalf("image URL = %q, want root-relative", body.Images[0].Url)
	}
}
