package httpapi

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/google/uuid"
	"github.com/tprei/sdds/services/api/internal/media"
)

var errInvalidImageUploadMultipart = errors.New("invalid image upload multipart body")

func parseImageUploadMultipart(r *http.Request, destination io.Writer) (string, error) {
	if r == nil || r.Body == nil || destination == nil {
		return "", errInvalidImageUploadMultipart
	}
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		return "", errInvalidImageUploadMultipart
	}

	reader := multipart.NewReader(r.Body, params["boundary"])
	var requestID string
	var seenID, seenFile bool
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil || part == nil {
			if nextErr == nil {
				nextErr = errInvalidImageUploadMultipart
			}
			return "", nextErr
		}
		name, filename, hasFilename, err := parseImageUploadPart(part)
		if err != nil {
			return "", err
		}
		switch name {
		case "upload_request_id":
			if seenID || hasFilename {
				return "", errInvalidImageUploadMultipart
			}
			value, err := readUploadRequestID(part)
			if err != nil {
				return "", err
			}
			parsed, err := uuid.Parse(value)
			if err != nil || parsed.String() != value {
				return "", media.ErrInvalidUploadRequest
			}
			requestID = value
			seenID = true
		case "file":
			if seenFile || !hasFilename || filename == "" {
				return "", errInvalidImageUploadMultipart
			}
			if _, err := io.Copy(destination, part); err != nil {
				return "", err
			}
			seenFile = true
		default:
			return "", errInvalidImageUploadMultipart
		}
	}
	if !seenID || !seenFile {
		return "", errInvalidImageUploadMultipart
	}
	return requestID, nil
}

func parseImageUploadPart(part *multipart.Part) (name, filename string, hasFilename bool, err error) {
	if part == nil {
		return "", "", false, errInvalidImageUploadMultipart
	}
	mediaType, params, parseErr := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if parseErr != nil || mediaType != "form-data" {
		return "", "", false, errInvalidImageUploadMultipart
	}
	name, ok := params["name"]
	if !ok || name == "" {
		return "", "", false, errInvalidImageUploadMultipart
	}
	filename, hasFilename = params["filename"]
	return name, filename, hasFilename, nil
}

func readUploadRequestID(part *multipart.Part) (string, error) {
	value, err := io.ReadAll(io.LimitReader(part, 128))
	if err != nil {
		return "", err
	}
	if len(value) == 0 {
		return "", media.ErrInvalidUploadRequest
	}
	var extra [1]byte
	for {
		count, readErr := part.Read(extra[:])
		if count > 0 {
			return "", media.ErrInvalidUploadRequest
		}
		if errors.Is(readErr, io.EOF) {
			return string(value), nil
		}
		if readErr != nil {
			return "", readErr
		}
	}
}
