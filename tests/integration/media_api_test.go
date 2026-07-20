//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/openapi"
)

const (
	mediaFixturePath        = "../fixtures/media/pao-de-queijo-640x427.jpg"
	mediaFixtureContentType = "image/jpeg"
	mediaFixtureWidth       = 640
	mediaFixtureHeight      = 427
	mediaFixtureByteSize    = 33887
	mediaFixtureSHA256      = "57e8aeda4753fca6b9077b271ccee6d6558923a72767b9c1bf3773e43254afbb"
)

func requireMediaAPIRuntimeBoundaries(t *testing.T, publicClient *openapi.ClientWithResponses, client *openapi.ClientWithResponses, author openapi.AuthorSummary) {
	t.Helper()

	fixture := loadMediaFixture(t)
	uploadRequestID := newMediaUUID(t)
	receipt := prepareImageUpload(t, client, uploadRequestID, fixture)
	requirePrivateBeforeAssociation(t, publicClient, receipt)

	replayedReceipt := prepareImageUpload(t, client, uploadRequestID, fixture)
	if !reflect.DeepEqual(receipt, replayedReceipt) {
		t.Fatalf("replayed upload receipt mismatch (-first +replay):\nfirst=%#v\nreplay=%#v", receipt, replayedReceipt)
	}

	imageUploadIDs := []string{receipt.ImageUploadId.String()}
	request := openapi.CreateNoteJSONRequestBody{
		Title:           "Pão de queijo da integração",
		Body:            "Imagem pública anexada ao cartão.",
		CategorySlug:    "food",
		ClientRequestId: "media-note-" + newMediaUUID(t),
		ImageUploadIds:  &imageUploadIDs,
	}
	created := createNote(t, client, request)
	createdImage := requireSingleImageMetadata(t, created, receipt)
	if !reflect.DeepEqual(author, created.Author) {
		t.Fatalf("created note author mismatch (-want +got):\nwant=%#v\ngot=%#v", author, created.Author)
	}

	replayedNote := createNote(t, client, request)
	if !reflect.DeepEqual(created, replayedNote) {
		t.Fatalf("replayed note mismatch (-first +replay):\nfirst=%#v\nreplay=%#v", created, replayedNote)
	}

	detail := getNote(t, publicClient, created.Id)
	if !reflect.DeepEqual(createdImage, requireSingleImageMetadata(t, detail, receipt)) {
		t.Fatalf("detail image metadata mismatch (-create +detail):\ncreate=%#v\ndetail=%#v", createdImage, detail.Images[0])
	}
	if !reflect.DeepEqual(author, detail.Author) {
		t.Fatalf("detail note author mismatch (-want +got):\nwant=%#v\ngot=%#v", author, detail.Author)
	}

	listed := findNoteByID(t, listNotes(t, publicClient), created.Id)
	if !reflect.DeepEqual(createdImage, requireSingleImageMetadata(t, listed, receipt)) {
		t.Fatalf("collection image metadata mismatch (-create +collection):\ncreate=%#v\ncollection=%#v", createdImage, listed.Images[0])
	}
	if !reflect.DeepEqual(author, listed.Author) {
		t.Fatalf("collection note author mismatch (-want +got):\nwant=%#v\ngot=%#v", author, listed.Author)
	}

	firstImage := getPublicMediaImage(t, publicClient, receipt)
	secondImage := getPublicMediaImage(t, publicClient, receipt)
	requireStableMediaResponse(t, firstImage, secondImage, fixture)
}

func loadMediaFixture(t *testing.T) []byte {
	t.Helper()

	fixture, err := os.ReadFile(mediaFixturePath)
	if err != nil {
		t.Fatalf("read media fixture %q: %v", mediaFixturePath, err)
	}
	if len(fixture) != mediaFixtureByteSize {
		t.Fatalf("media fixture byte size = %d, want %d", len(fixture), mediaFixtureByteSize)
	}
	digest := sha256.Sum256(fixture)
	if got := hex.EncodeToString(digest[:]); got != mediaFixtureSHA256 {
		t.Fatalf("media fixture SHA-256 = %s, want %s", got, mediaFixtureSHA256)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(fixture))
	if err != nil {
		t.Fatalf("decode media fixture JPEG config: %v", err)
	}
	if config.Width != mediaFixtureWidth || config.Height != mediaFixtureHeight {
		t.Fatalf("media fixture dimensions = %dx%d, want %dx%d", config.Width, config.Height, mediaFixtureWidth, mediaFixtureHeight)
	}
	return fixture
}

func prepareImageUpload(t *testing.T, client *openapi.ClientWithResponses, uploadRequestID string, fixture []byte) openapi.ImageUploadReceipt {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writeMultipartField(writer, "upload_request_id", uploadRequestID); err != nil {
		t.Fatalf("write upload_request_id multipart field: %v", err)
	}
	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", `form-data; name="file"; filename="pao-de-queijo-640x427.jpg"`)
	fileHeader.Set("Content-Type", mediaFixtureContentType)
	filePart, err := writer.CreatePart(fileHeader)
	if err != nil {
		t.Fatalf("create image multipart part: %v", err)
	}
	if _, err := filePart.Write(fixture); err != nil {
		t.Fatalf("write image multipart part: %v", err)
	}
	contentType := writer.FormDataContentType()
	if err := writer.Close(); err != nil {
		t.Fatalf("close image multipart writer: %v", err)
	}

	response, err := client.PrepareImageUploadWithBodyWithResponse(context.Background(), contentType, bytes.NewReader(body.Bytes()))
	if err != nil {
		t.Fatalf("POST /v1/media/image-uploads: %v", err)
	}
	requireStatus(t, "POST /v1/media/image-uploads", response.StatusCode(), http.StatusCreated, response.Body)
	if response.JSON201 == nil {
		t.Fatal("POST /v1/media/image-uploads returned 201 without JSON body")
	}
	requireUploadReceipt(t, *response.JSON201)
	return *response.JSON201
}

func writeMultipartField(writer *multipart.Writer, name string, value string) error {
	field, err := writer.CreateFormField(name)
	if err != nil {
		return err
	}
	_, err = field.Write([]byte(value))
	return err
}

func requirePrivateBeforeAssociation(t *testing.T, publicClient *openapi.ClientWithResponses, receipt openapi.ImageUploadReceipt) {
	t.Helper()

	response, err := publicClient.GetMediaImageWithResponse(context.Background(), receipt.ImageUploadId)
	if err != nil {
		t.Fatalf("GET staged /v1/media/images/{image_id}: %v", err)
	}
	requireStatus(t, "GET staged /v1/media/images/{image_id}", response.StatusCode(), http.StatusNotFound, response.Body)
	if response.JSON404 == nil {
		t.Fatal("GET staged /v1/media/images/{image_id} returned 404 without JSON body")
	}
	if response.JSON404.Code != openapi.ErrorCodeNotFound {
		t.Fatalf("staged image error code = %s, want %s", response.JSON404.Code, openapi.ErrorCodeNotFound)
	}
}

func requireUploadReceipt(t *testing.T, receipt openapi.ImageUploadReceipt) {
	t.Helper()

	imageID := receipt.ImageUploadId.String()
	if imageID == "00000000-0000-0000-0000-000000000000" {
		t.Fatal("upload receipt image_upload_id is nil UUID")
	}
	if receipt.ContentType != openapi.ImageUploadReceiptContentTypeImagejpeg {
		t.Fatalf("upload receipt content_type = %q, want %q", receipt.ContentType, mediaFixtureContentType)
	}
	if receipt.ByteSize != mediaFixtureByteSize {
		t.Fatalf("upload receipt byte_size = %d, want %d", receipt.ByteSize, mediaFixtureByteSize)
	}
	if receipt.Width != mediaFixtureWidth || receipt.Height != mediaFixtureHeight {
		t.Fatalf("upload receipt dimensions = %dx%d, want %dx%d", receipt.Width, receipt.Height, mediaFixtureWidth, mediaFixtureHeight)
	}
	if receipt.ExpiresAt <= time.Now().UnixMilli() {
		t.Fatalf("upload receipt expires_at = %d, want future timestamp", receipt.ExpiresAt)
	}
}

func requireSingleImageMetadata(t *testing.T, note openapi.Note, receipt openapi.ImageUploadReceipt) openapi.NoteImage {
	t.Helper()

	if len(note.Images) != 1 {
		t.Fatalf("note image count = %d, want 1", len(note.Images))
	}
	image := note.Images[0]
	imageID := receipt.ImageUploadId.String()
	if image.Id != imageID {
		t.Fatalf("note image id = %q, want %q", image.Id, imageID)
	}
	if image.Url != "/v1/media/images/"+imageID {
		t.Fatalf("note image url = %q, want %q", image.Url, "/v1/media/images/"+imageID)
	}
	if image.ContentType != openapi.NoteImageContentTypeImagejpeg {
		t.Fatalf("note image content_type = %q, want %q", image.ContentType, mediaFixtureContentType)
	}
	if image.ByteSize != mediaFixtureByteSize {
		t.Fatalf("note image byte_size = %d, want %d", image.ByteSize, mediaFixtureByteSize)
	}
	if image.Width != mediaFixtureWidth || image.Height != mediaFixtureHeight {
		t.Fatalf("note image dimensions = %dx%d, want %dx%d", image.Width, image.Height, mediaFixtureWidth, mediaFixtureHeight)
	}
	if image.Position != 0 {
		t.Fatalf("note image position = %d, want 0", image.Position)
	}
	if image.CreatedAt <= 0 || image.UpdatedAt <= 0 {
		t.Fatalf("note image timestamps = %d/%d, want positive values", image.CreatedAt, image.UpdatedAt)
	}
	if image.CreatedAt > image.UpdatedAt {
		t.Fatalf("note image timestamps = %d/%d, want created_at <= updated_at", image.CreatedAt, image.UpdatedAt)
	}
	return image
}

func findNoteByID(t *testing.T, notes openapi.ListNotesResponse, id string) openapi.Note {
	t.Helper()

	for _, note := range notes.Notes {
		if note.Id == id {
			return note
		}
	}
	t.Fatalf("note id %q missing from collection", id)
	return openapi.Note{}
}

func getPublicMediaImage(t *testing.T, publicClient *openapi.ClientWithResponses, receipt openapi.ImageUploadReceipt) openapi.GetMediaImageHTTPResponse {
	t.Helper()

	response, err := publicClient.GetMediaImageWithResponse(context.Background(), receipt.ImageUploadId)
	if err != nil {
		t.Fatalf("GET /v1/media/images/{image_id}: %v", err)
	}
	return *response
}

func requireStableMediaResponse(t *testing.T, first openapi.GetMediaImageHTTPResponse, second openapi.GetMediaImageHTTPResponse, fixture []byte) {
	t.Helper()

	wantHeaders := map[string]string{
		"Cache-Control":          "public, max-age=31536000, immutable",
		"Content-Disposition":    "inline",
		"Content-Type":           mediaFixtureContentType,
		"Content-Length":         strconv.Itoa(mediaFixtureByteSize),
		"ETag":                   `"` + mediaFixtureSHA256 + `"`,
		"X-Content-Type-Options": "nosniff",
	}
	responses := []openapi.GetMediaImageHTTPResponse{first, second}
	for index, response := range responses {
		requireStatus(t, fmt.Sprintf("GET /v1/media/images/{image_id} #%d", index+1), response.StatusCode(), http.StatusOK, response.Body)
		if response.HTTPResponse == nil {
			t.Fatalf("GET /v1/media/images/{image_id} #%d has nil HTTP response", index+1)
		}
		if !bytes.Equal(response.Body, fixture) {
			t.Fatalf("GET /v1/media/images/{image_id} #%d body differs from committed fixture", index+1)
		}
		if got := len(response.Body); got != mediaFixtureByteSize {
			t.Fatalf("GET /v1/media/images/{image_id} #%d body length = %d, want %d", index+1, got, mediaFixtureByteSize)
		}
		digest := sha256.Sum256(response.Body)
		if got := hex.EncodeToString(digest[:]); got != mediaFixtureSHA256 {
			t.Fatalf("GET /v1/media/images/{image_id} #%d SHA-256 = %s, want %s", index+1, got, mediaFixtureSHA256)
		}
		if response.HTTPResponse.ContentLength != mediaFixtureByteSize {
			t.Fatalf("GET /v1/media/images/{image_id} #%d response content length = %d, want %d", index+1, response.HTTPResponse.ContentLength, mediaFixtureByteSize)
		}
		for name, want := range wantHeaders {
			if got := response.HTTPResponse.Header.Get(name); got != want {
				t.Fatalf("GET /v1/media/images/{image_id} #%d %s = %q, want %q", index+1, name, got, want)
			}
		}
	}
	if !bytes.Equal(first.Body, second.Body) {
		t.Fatal("repeated GET /v1/media/images/{image_id} bodies differ")
	}
	for name := range wantHeaders {
		firstValue := first.HTTPResponse.Header.Get(name)
		secondValue := second.HTTPResponse.Header.Get(name)
		if firstValue != secondValue {
			t.Fatalf("repeated GET /v1/media/images/{image_id} %s differs: %q vs %q", name, firstValue, secondValue)
		}
	}
}

func newMediaUUID(t *testing.T) string {
	t.Helper()

	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatalf("generate UUID: %v", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}
