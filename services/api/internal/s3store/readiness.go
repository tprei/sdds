package s3store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/tprei/sdds/services/api/internal/media"
)

const (
	readinessSentinelKey            media.ObjectKey = "system/readiness"
	readinessSentinelPayload                        = "sdds-media-ready-v1\n"
	readinessSentinelContentLength  int64           = int64(len(readinessSentinelPayload))
	readinessSentinelDigest                         = "5aff33ce5e386989939a8a504923897432db5b5a818518ccd876dadf2ad7398f"
	readinessSentinelChecksumSHA256                 = "Wv8zzl44aYmTmopQSSOJdDLbW1qBhRjM2Hba3yrXOY8="
)

func (store *Store) VerifyReadiness(ctx context.Context) error {
	requestCtx, cancel := store.withTimeout(ctx)
	defer cancel()
	output, err := store.client.HeadObject(requestCtx, &s3.HeadObjectInput{
		Bucket:       aws.String(store.bucket),
		Key:          aws.String(string(readinessSentinelKey)),
		ChecksumMode: s3types.ChecksumModeEnabled,
	})
	if err != nil {
		return mapProviderError(err)
	}
	if output == nil || output.ContentLength == nil || *output.ContentLength != readinessSentinelContentLength ||
		output.ChecksumSHA256 == nil || *output.ChecksumSHA256 != readinessSentinelChecksumSHA256 {
		return media.ErrObjectIntegrity
	}
	digest, ok := metadataDigest(output.Metadata)
	if !ok || digest != readinessSentinelDigest || len(output.Metadata) != 1 {
		return media.ErrObjectIntegrity
	}
	return nil
}
