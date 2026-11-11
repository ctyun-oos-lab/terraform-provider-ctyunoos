// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithymw "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
)

const (
	oosBucketCreateXMLNS           = "http://s3.amazonaws.com/doc/2006-03-01/"
	errCodeBucketAlreadyExists     = "BucketAlreadyExists"
	errCodeBucketAlreadyOwnedByYou = "BucketAlreadyOwnedByYou"
	errCodeNoSuchBucket            = "NoSuchBucket"
)

type oosSharedBucketState struct {
	once                    sync.Once
	softDeleteOnce          sync.Once
	softDeletePurposeOnce   sync.Once
	bucket                  string
	err                     error
	softDeleteConfigErr     error
	softDeleteBucketPurpose string
}

var (
	oosSharedObjectBucketState                     oosSharedBucketState
	oosSharedObjectCopySourceState                 oosSharedBucketState
	oosSharedObjectCopyTargetState                 oosSharedBucketState
	oosSharedObjectDataSourceExpirationBucketState oosSharedBucketState
	oosSharedObjectDataSourceEncryptionBucketState oosSharedBucketState
	oosSharedObjectRecoverWindowBucketState        oosSharedBucketState
	oosSharedObjectRecoverAfterBucketState         oosSharedBucketState
	oosSharedObjectRecoverBeforeBucketState        oosSharedBucketState
	oosSharedObjectRecoverRangeBucketState         oosSharedBucketState
	oosSharedObjectRecoverEncodingBucketState      oosSharedBucketState
	oosSharedObjectsSoftDeleteBucketState          oosSharedBucketState
)

type oosCreateBucketConfiguration struct {
	XMLName xml.Name `xml:"CreateBucketConfiguration"`
	XMLNS   string   `xml:"xmlns,attr,omitempty"`
}

func OOSSharedObjectBucket(ctx context.Context, t *testing.T) string {
	t.Helper()
	return oosSharedSoftDeleteBucket(ctx, t, &oosSharedObjectBucketState, "object")
}

func OOSSharedObjectCopySourceBucket(ctx context.Context, t *testing.T) string {
	t.Helper()
	return oosSharedSoftDeleteBucket(ctx, t, &oosSharedObjectCopySourceState, "object-copy-source")
}

func OOSSharedObjectCopyTargetBucket(ctx context.Context, t *testing.T) string {
	t.Helper()
	return oosSharedSoftDeleteBucket(ctx, t, &oosSharedObjectCopyTargetState, "object-copy-target")
}

// OOSSharedObjectDataSourceExpirationBucket returns a dedicated shared bucket
// for object data source lifecycle-expiration tests.
func OOSSharedObjectDataSourceExpirationBucket(ctx context.Context, t *testing.T) string {
	t.Helper()
	return oosSharedSoftDeleteBucket(ctx, t, &oosSharedObjectDataSourceExpirationBucketState, "object-data-source-expiration")
}

// OOSSharedObjectDataSourceEncryptionBucket returns a dedicated shared bucket
// for object data source default-encryption tests.
func OOSSharedObjectDataSourceEncryptionBucket(ctx context.Context, t *testing.T) string {
	t.Helper()
	return oosSharedSoftDeleteBucket(ctx, t, &oosSharedObjectDataSourceEncryptionBucketState, "object-data-source-encryption")
}
func OOSSharedObjectRecoverWindowBucket(ctx context.Context, t *testing.T) string {
	t.Helper()
	return oosSharedSoftDeleteBucket(ctx, t, &oosSharedObjectRecoverWindowBucketState, "object-recover-window")
}

func OOSSharedObjectRecoverAfterBucket(ctx context.Context, t *testing.T) string {
	t.Helper()
	return oosSharedSoftDeleteBucket(ctx, t, &oosSharedObjectRecoverAfterBucketState, "object-recover-after")
}

func OOSSharedObjectRecoverBeforeBucket(ctx context.Context, t *testing.T) string {
	t.Helper()
	return oosSharedSoftDeleteBucket(ctx, t, &oosSharedObjectRecoverBeforeBucketState, "object-recover-before")
}

func OOSSharedObjectRecoverRangeBucket(ctx context.Context, t *testing.T) string {
	t.Helper()
	return oosSharedSoftDeleteBucket(ctx, t, &oosSharedObjectRecoverRangeBucketState, "object-recover-range")
}

func OOSSharedObjectRecoverEncodingBucket(ctx context.Context, t *testing.T) string {
	t.Helper()
	return oosSharedSoftDeleteBucket(ctx, t, &oosSharedObjectRecoverEncodingBucketState, "object-recover-encoding")
}

func OOSSharedObjectsSoftDeleteBucket(ctx context.Context, t *testing.T) string {
	t.Helper()
	return oosSharedSoftDeleteBucket(ctx, t, &oosSharedObjectsSoftDeleteBucketState, "objects-soft-delete")
}

// OOSSharedObjectsSoftDeleteScenarioBucket returns a unique soft-delete bucket
// for an acceptance-test scenario, preventing stale objects across reruns.
// The bucket-name purpose is intentionally short to stay within S3 name limits.
func OOSSharedObjectsSoftDeleteScenarioBucket(ctx context.Context, t *testing.T, purpose string) string {
	t.Helper()
	state := &oosSharedBucketState{}
	scenario := purpose
	if len(scenario) > 8 {
		scenario = scenario[:8]
	}
	uniquePurpose := fmt.Sprintf("osd-%s-%s", scenario, RandomWithPrefix(t, "x"))
	return oosSharedSoftDeleteBucket(ctx, t, state, uniquePurpose)
}

type oosSharedSoftDeleteConfiguration struct {
	XMLName xml.Name `xml:"SoftDeleteConfiguration"`
	XMLNS   string   `xml:"xmlns,attr,omitempty"`
	Days    int32    `xml:"Days"`
}

func oosSharedSoftDeleteBucket(ctx context.Context, t *testing.T, state *oosSharedBucketState, purpose string) string {
	t.Helper()

	// Keep a dedicated bucket stable within a test process while avoiding reuse
	// of soft-deleted objects from an earlier acceptance-test run.
	state.softDeletePurposeOnce.Do(func() {
		state.softDeleteBucketPurpose = fmt.Sprintf("%s-%08x", purpose, uint32(RandInt(t)))
	})
	bucket := oosSharedBucket(ctx, t, state, state.softDeleteBucketPurpose)
	state.softDeleteOnce.Do(func() {
		body, err := xml.Marshal(oosSharedSoftDeleteConfiguration{
			XMLNS: oosBucketCreateXMLNS,
			Days:  1,
		})
		if err != nil {
			state.softDeleteConfigErr = fmt.Errorf("marshaling shared bucket soft delete configuration (%s): %w", bucket, err)
			return
		}

		conn := ProviderMeta(ctx, t).S3Client(ctx)
		_, err = conn.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}, func(o *s3.Options) {
			o.APIOptions = append(o.APIOptions,
				rewriteOOSSharedBucketQueryMiddleware("soft-delete"),
				replaceRequestBodyMiddleware(body),
			)
		})
		if err != nil {
			state.softDeleteConfigErr = fmt.Errorf("enabling soft delete for shared OOS bucket (%s): %w", bucket, err)
		}
	})
	if state.softDeleteConfigErr != nil {
		t.Fatal(state.softDeleteConfigErr)
	}

	return bucket
}
func rewriteOOSSharedBucketQueryMiddleware(rawQuery string) func(*smithymw.Stack) error {
	return func(stack *smithymw.Stack) error {
		return stack.Build.Add(
			smithymw.BuildMiddlewareFunc("OOSSharedBucketRewriteQuery", func(ctx context.Context, in smithymw.BuildInput, next smithymw.BuildHandler) (smithymw.BuildOutput, smithymw.Metadata, error) {
				if req, ok := in.Request.(*smithyhttp.Request); ok {
					req.URL.RawQuery = rawQuery
				}
				return next.HandleBuild(ctx, in)
			}),
			smithymw.After,
		)
	}
}
func oosSharedBucket(ctx context.Context, t *testing.T, state *oosSharedBucketState, purpose string) string {
	t.Helper()

	state.once.Do(func() {
		PreCheck(ctx, t)

		bucket, err := ensureOOSSharedBucket(ctx, t, purpose)
		state.bucket = bucket
		state.err = err
	})

	if state.err != nil {
		t.Fatalf("ensuring shared OOS bucket (%s): %s", purpose, state.err)
	}

	return state.bucket
}

func ensureOOSSharedBucket(ctx context.Context, t *testing.T, purpose string) (string, error) {
	t.Helper()

	client := ProviderMeta(ctx, t)
	conn := client.S3Client(ctx)
	bucket := sharedOOSBucketName(client.AccountID(ctx), purpose)

	_, err := conn.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return bucket, nil
	}

	if !tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) && !tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) && !retry.NotFound(err) {
		return "", fmt.Errorf("checking shared bucket (%s): %w", bucket, err)
	}

	body, err := xml.Marshal(oosCreateBucketConfiguration{
		XMLNS: oosBucketCreateXMLNS,
	})
	if err != nil {
		return "", fmt.Errorf("marshaling shared bucket create body: %w", err)
	}

	_, err = conn.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, replaceRequestBodyMiddleware(body))
	})
	if err != nil && !tfawserr.ErrCodeEquals(err, errCodeBucketAlreadyOwnedByYou, errCodeBucketAlreadyExists) {
		return "", fmt.Errorf("creating shared bucket (%s): %w", bucket, err)
	}

	return bucket, nil
}

func sharedOOSBucketName(accountID, purpose string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.Join([]string{
		accountID,
		purpose,
		OOSMetadataLocation(),
		os.Getenv("CTYUNOOS_ENDPOINT"),
		Region(),
	}, "|")))

	hashSuffix := hex.EncodeToString(h.Sum(nil))[:8]
	return fmt.Sprintf("tf-acc-oos-%s-%s", purpose, hashSuffix)
}

func replaceRequestBodyMiddleware(body []byte) func(*smithymw.Stack) error {
	return func(stack *smithymw.Stack) error {
		return stack.Serialize.Add(
			smithymw.SerializeMiddlewareFunc("OOSSharedBucketReplaceRequestBody",
				func(ctx context.Context, in smithymw.SerializeInput, next smithymw.SerializeHandler) (smithymw.SerializeOutput, smithymw.Metadata, error) {
					req, ok := in.Request.(*smithyhttp.Request)
					if !ok {
						return smithymw.SerializeOutput{}, smithymw.Metadata{}, fmt.Errorf("unexpected transport type %T", in.Request)
					}

					req, err := req.SetStream(bytes.NewReader(body))
					if err != nil {
						return smithymw.SerializeOutput{}, smithymw.Metadata{}, err
					}
					in.Request = req

					return next.HandleSerialize(ctx, in)
				}),
			smithymw.After,
		)
	}
}
