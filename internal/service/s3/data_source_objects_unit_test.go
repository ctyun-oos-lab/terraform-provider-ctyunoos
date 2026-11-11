// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
)

// TC-OOS-DOS-10 Missing NextMarker Negative verifies that a truncated response missing NextMarker returns a clear error
// instead of silently keeping an incomplete object list.
func TestUnitS3ObjectsDataSource_MissingNextMarker(t *testing.T) {
	t.Parallel()

	got, err := tfs3.ValidateObjectsNextMarker("test-bucket", &awss3.ListObjectsOutput{
		IsTruncated: aws.Bool(true),
	})
	if err == nil {
		t.Fatal("expected an error for a truncated response without NextMarker")
	}
	if got != "" {
		t.Fatalf("expected an empty marker on error, got %q", got)
	}
	if want := "listing S3 Bucket (test-bucket) Objects: truncated response missing NextMarker"; err.Error() != want {
		t.Fatalf("unexpected error: %q", err)
	}
}

// Verifies that a truncated response carrying NextMarker is parsed correctly and returns the marker.
func TestUnitS3ObjectsDataSource_NextMarker(t *testing.T) {
	t.Parallel()

	const wantMarker = "prefix2/0"

	got, err := tfs3.ValidateObjectsNextMarker("test-bucket", &awss3.ListObjectsOutput{
		IsTruncated: aws.Bool(true),
		NextMarker:  aws.String(wantMarker),
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got != wantMarker {
		t.Fatalf("expected marker %q, got %q", wantMarker, got)
	}
}

// Verifies that a non-truncated response does not require NextMarker and returns an empty marker.
func TestUnitS3ObjectsDataSource_NotTruncated(t *testing.T) {
	t.Parallel()

	got, err := tfs3.ValidateObjectsNextMarker("test-bucket", &awss3.ListObjectsOutput{
		IsTruncated: aws.Bool(false),
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got != "" {
		t.Fatalf("expected an empty marker, got %q", got)
	}
}
