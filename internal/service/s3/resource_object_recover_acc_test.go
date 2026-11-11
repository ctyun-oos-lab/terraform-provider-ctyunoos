// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	tfresource "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-S3-OBJECT-RECOVER-08 Input Validation - plain key without encoding_type
func TestAccS3ObjectRecover_InputValidation_encodingTypeURL(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "ctyun_oos_object_recover.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "recover key+url")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					testAccPutRecoverObject(ctx, t, bucket, key)
					testAccDeleteRecoverObject(ctx, t, bucket, key)
				},
				Config: testAccObjectRecoverConfig_recoverWithEncodingType(bucket, "url", key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "encoding_type", "url"),
					resource.TestCheckResourceAttr(resourceName, "recovered_keys.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "recovered_keys.0", key),
					resource.TestCheckResourceAttr(resourceName, "errors.#", "0"),
					testAccCheckRecoveredObjectExists(ctx, t, bucket, key),
				),
			},
		},
	})
}

// TC-S3-OBJECT-RECOVER-08 Input Validation - plain key without encoding_type
// TestAccS3ObjectRecover_InputValidation_plainKeyWithoutEncoding verifies that without encoding_type, both successfully and unsuccessfully recovered plain and special keys keep their original key.
func TestAccS3ObjectRecover_InputValidation_plainKeyWithoutEncoding(t *testing.T) {
	testAccObjectRecoverEncodingType(t, "plain-key-without-encoding", "")
}

// TC-S3-OBJECT-RECOVER-08 Input Validation - plain key with encoding_type
// TestAccS3ObjectRecover_InputValidation_plainKeyWithEncoding verifies that with encoding_type=url, both successfully and unsuccessfully recovered plain and special keys keep their original key.
func TestAccS3ObjectRecover_InputValidation_plainKeyWithEncoding(t *testing.T) {
	testAccObjectRecoverEncodingType(t, "plain-key-with-encoding", "url")
}

// TC-S3-OBJECT-RECOVER-08 Input Validation - special key with encoding_type
// TestAccS3ObjectRecover_InputValidation_specialKeyWithEncoding verifies that with encoding_type=url, special keys are decoded back to the original key in both the success state and the failure diagnostics.
func TestAccS3ObjectRecover_InputValidation_specialKeyWithEncoding(t *testing.T) {
	testAccObjectRecoverEncodingType(t, "special-key-with-encoding", "url")
}

// TC-S3-OBJECT-RECOVER-08 Input Validation - special key without encoding_type
// TestAccS3ObjectRecover_InputValidation_specialKeyWithoutEncoding verifies that without encoding_type, special keys keep their original key in both the success state and the failure diagnostics.
func TestAccS3ObjectRecover_InputValidation_specialKeyWithoutEncoding(t *testing.T) {
	testAccObjectRecoverEncodingType(t, "special-key-without-encoding", "")
}

func testAccObjectRecoverEncodingType(t *testing.T, name, encodingType string) {
	t.Helper()

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-object-recover-"+name)
	bucket := acctest.OOSSharedObjectRecoverEncodingBucket(ctx, t)
	recoveredPlainKey := testAccObjectKey(rName, "plain-recovered")
	recoveredSpecialKey := testAccObjectKey(rName, "special recovered")
	failedPlainKey := testAccObjectKey(rName, "plain-failed")
	failedSpecialKey := testAccObjectKey(rName, "special failed")
	successResourceName := "ctyun_oos_object_recover.success"

	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(successResourceName, names.AttrBucket, bucket),
		resource.TestCheckResourceAttr(successResourceName, "recover.#", "2"),
		resource.TestCheckResourceAttr(successResourceName, "recover.0.key", recoveredPlainKey),
		resource.TestCheckResourceAttr(successResourceName, "recover.1.key", recoveredSpecialKey),
		resource.TestCheckResourceAttr(successResourceName, "recovered_keys.#", "2"),
		resource.TestCheckResourceAttr(successResourceName, "errors.#", "0"),
		testAccCheckRecoveredKeysContains(successResourceName, recoveredPlainKey),
		testAccCheckRecoveredKeysContains(successResourceName, recoveredSpecialKey),
		testAccCheckObjectRecoverReadable(ctx, t, bucket, recoveredPlainKey),
		testAccCheckObjectRecoverReadable(ctx, t, bucket, recoveredSpecialKey),
	}
	if encodingType != "" {
		checks = append(checks, resource.TestCheckResourceAttr(successResourceName, "encoding_type", encodingType))
	}

	errorRegexp := regexp.MustCompile(fmt.Sprintf(
		`(?s)recovering OOS Objects in Bucket \(%s\): .*?(?:%s.*%s|%s.*%s)`,
		regexp.QuoteMeta(bucket),
		regexp.QuoteMeta(failedPlainKey), regexp.QuoteMeta(failedSpecialKey),
		regexp.QuoteMeta(failedSpecialKey), regexp.QuoteMeta(failedPlainKey),
	))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					for _, key := range []string{recoveredPlainKey, recoveredSpecialKey, failedPlainKey, failedSpecialKey} {
						testAccPutObjectForRecover(ctx, t, bucket, key)
					}
					for _, key := range []string{recoveredPlainKey, recoveredSpecialKey} {
						testAccSoftDeleteObjectForRecover(ctx, t, bucket, key)
					}
				},
				Config: testAccObjectRecoverConfig_namedRecoverWithEncodingType("success", bucket, encodingType, recoveredPlainKey, recoveredSpecialKey),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				Config:      testAccObjectRecoverConfig_namedRecoverWithEncodingType("failed", bucket, encodingType, failedPlainKey, failedSpecialKey),
				ExpectError: errorRegexp,
			},
		},
	})
}

// TC-S3-OBJECT-RECOVER-08 Input Validation - encoding_type case insensitive
// TestAccS3ObjectRecover_EncodingType_caseInsensitive verifies that objects are recovered normally with any casing of encoding_type.
func TestAccS3ObjectRecover_EncodingType_caseInsensitive(t *testing.T) {
	for _, encodingType := range []string{"URL", "Url", "uRl"} {
		t.Run(encodingType, func(t *testing.T) {
			testAccObjectRecoverEncodingType(t, "encoding-type-"+strings.ToLower(encodingType), encodingType)
		})
	}
}

// TC-S3-OBJECT-RECOVER-02 Recover Multiple Objects - partial result
// TestAccS3ObjectRecover_partialFailure verifies that a recover containing both soft-deleted and non-deleted objects returns a determinable error.
func TestAccS3ObjectRecover_RecoverMultipleObjects_liveKey(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	recoverKey := testAccObjectKey(rName, "recover-key")
	liveKey := testAccObjectKey(rName, "live-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					testAccPutRecoverObject(ctx, t, bucket, recoverKey)
					testAccPutRecoverObject(ctx, t, bucket, liveKey)
					testAccDeleteRecoverObject(ctx, t, bucket, recoverKey)
				},
				Config:      testAccObjectRecoverConfig_recover(bucket, recoverKey, liveKey),
				ExpectError: regexp.MustCompile(fmt.Sprintf(`recovering OOS Objects in Bucket \(%s\): .*%s`, regexp.QuoteMeta(bucket), regexp.QuoteMeta(liveKey))),
			},
		},
	})
}

// TestAccS3ObjectRecover_window verifies that only objects within the soft-delete time window are recovered.
func testAccPutRecoverObject(ctx context.Context, t *testing.T, bucket, key string) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
	_, err := conn.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String("text/plain"),
		Body:        strings.NewReader("recover me"),
	}, testAccOOSPutObjectOptions)
	if err != nil {
		t.Fatalf("putting recover object (%s/%s): %s", bucket, key, err)
	}
}

func testAccDeleteRecoverObject(ctx context.Context, t *testing.T, bucket, key string) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
	_, err := conn.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("deleting recover object (%s/%s): %s", bucket, key, err)
	}

	_, err = tfresource.RetryUntilNotFound(ctx, tfs3.BucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return tfs3.FindObjectByBucketAndKey(ctx, conn, bucket, key, "", "")
	})
	if err != nil {
		t.Fatalf("waiting for recover object (%s/%s) soft delete visibility: %s", bucket, key, err)
	}
}

func testAccCheckRecoveredObjectExists(ctx context.Context, t *testing.T, bucket, key string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		_, err := tfresource.RetryWhenNotFound(ctx, tfs3.BucketPropagationTimeout, func(ctx context.Context) (*s3.HeadObjectOutput, error) {
			return tfs3.FindObjectByBucketAndKey(ctx, conn, bucket, key, "", "")
		})
		return err
	}
}

func testAccCheckRecoveredKeysContains(resourceName, key string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		countRaw, ok := rs.Primary.Attributes["recovered_keys.#"]
		if !ok {
			return fmt.Errorf("Attribute recovered_keys.# not found")
		}

		count, err := strconv.Atoi(countRaw)
		if err != nil {
			return fmt.Errorf("parsing recovered_keys.# %q: %w", countRaw, err)
		}

		for i := range count {
			if rs.Primary.Attributes[fmt.Sprintf("recovered_keys.%d", i)] == key {
				return nil
			}
		}

		return fmt.Errorf("recovered_keys does not contain %q", key)
	}
}

func testAccObjectRecoverConfig_recover(bucket string, recoverKeys ...string) string {
	recoverBlocks := make([]string, 0, len(recoverKeys))
	for _, key := range recoverKeys {
		recoverBlocks = append(recoverBlocks, fmt.Sprintf(`
  recover {
    key = %q
  }`, key))
	}

	return fmt.Sprintf(`
resource "ctyun_oos_object_recover" "test" {
  bucket = %q
%s
}
`, bucket, strings.Join(recoverBlocks, ""))
}

func testAccObjectRecoverConfig_recoverWithEncodingType(bucket, encodingType string, recoverKeys ...string) string {
	recoverBlocks := make([]string, 0, len(recoverKeys))
	for _, key := range recoverKeys {
		recoverBlocks = append(recoverBlocks, fmt.Sprintf(`
  recover {
    key = %q
  }`, key))
	}

	return fmt.Sprintf(`
resource "ctyun_oos_object_recover" "test" {
  bucket        = %q
  encoding_type = %q
%s
}
`, bucket, encodingType, strings.Join(recoverBlocks, ""))
}

// TC-S3-OBJECT-RECOVER-01 Recover Single Object
// TestAccS3ObjectRecover_RecoverSingleObject_basic uses a pre-provisioned shared bucket with soft delete enabled, creates a soft-deleted object, recovers it by key, and verifies the object is readable.
func TestAccS3ObjectRecover_RecoverSingleObject_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-object-recover")
	bucket := acctest.OOSSharedObjectsSoftDeleteBucket(ctx, t)
	key := testAccObjectKey(rName, "recover-key")
	resourceName := "ctyun_oos_object_recover.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					testAccPutObjectForRecover(ctx, t, bucket, key)
					testAccSoftDeleteObjectForRecover(ctx, t, bucket, key)
				},
				Config: testAccObjectRecoverConfig_recover(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(resourceName, "recover.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "recover.0.key", key),
					resource.TestCheckResourceAttr(resourceName, "recovered_keys.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "recovered_keys.0", key),
					resource.TestCheckResourceAttr(resourceName, "errors.#", "0"),
					testAccCheckObjectRecoverReadable(ctx, t, bucket, key),
				),
			},
		},
	})
}

// TC-S3-OBJECT-RECOVER-02 Recover Multiple Objects
// TestAccS3ObjectRecover_RecoverMultipleObjects_all restores every requested soft-deleted key.
func TestAccS3ObjectRecover_RecoverMultipleObjects_all(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-object-recover")
	bucket := acctest.OOSSharedObjectsSoftDeleteBucket(ctx, t)
	recoverKeys := []string{
		testAccObjectKey(rName, "recover-key-1"),
		testAccObjectKey(rName, "recover-key-2"),
		testAccObjectKey(rName, "recover-key-3"),
	}
	resourceName := "ctyun_oos_object_recover.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					for _, key := range recoverKeys {
						testAccPutObjectForRecover(ctx, t, bucket, key)
						testAccSoftDeleteObjectForRecover(ctx, t, bucket, key)
					}
				},
				Config: testAccObjectRecoverConfig_recover(bucket, recoverKeys...),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(resourceName, "recover.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "recover.0.key", recoverKeys[0]),
					resource.TestCheckResourceAttr(resourceName, "recover.1.key", recoverKeys[1]),
					resource.TestCheckResourceAttr(resourceName, "recover.2.key", recoverKeys[2]),
					resource.TestCheckResourceAttr(resourceName, "recovered_keys.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "errors.#", "0"),
					testAccCheckRecoveredKeysContains(resourceName, recoverKeys[0]),
					testAccCheckRecoveredKeysContains(resourceName, recoverKeys[1]),
					testAccCheckRecoveredKeysContains(resourceName, recoverKeys[2]),
					testAccCheckObjectRecoverReadable(ctx, t, bucket, recoverKeys[0]),
					testAccCheckObjectRecoverReadable(ctx, t, bucket, recoverKeys[1]),
					testAccCheckObjectRecoverReadable(ctx, t, bucket, recoverKeys[2]),
				),
			},
		},
	})
}

// TC-S3-OBJECT-RECOVER-02 Recover Multiple Objects - partial result
// TestAccS3ObjectRecover_RecoverMultipleObjects_missingKey verifies that when any key fails to recover, the recover resource's apply fails as a whole, even if OOS has already recovered the other requested objects.
func TestAccS3ObjectRecover_RecoverMultipleObjects_missingKey(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-object-recover")
	bucket := acctest.OOSSharedObjectsSoftDeleteBucket(ctx, t)
	recoverKey := testAccObjectKey(rName, "recover-key")
	missingKey := testAccObjectKey(rName, "missing-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					testAccPutObjectForRecover(ctx, t, bucket, recoverKey)
					testAccSoftDeleteObjectForRecover(ctx, t, bucket, recoverKey)
				},
				Config:      testAccObjectRecoverConfig_recover(bucket, recoverKey, missingKey),
				ExpectError: regexp.MustCompile(fmt.Sprintf(`recovering OOS Objects in Bucket \(%s\): .*%s`, regexp.QuoteMeta(bucket), regexp.QuoteMeta(missingKey))),
			},
			{
				Config: "terraform {}",
				Check:  testAccCheckObjectRecoverReadable(ctx, t, bucket, recoverKey),
			},
		},
	})
}

// TC-S3-OBJECT-RECOVER-03 Recover By Time Window - after only
// TestAccS3ObjectRecover_RecoverByTimeWindow_after restores only objects soft-deleted at or after the selector.
func TestAccS3ObjectRecover_RecoverByTimeWindow_after(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-object-recover")
	bucket := acctest.OOSSharedObjectRecoverAfterBucket(ctx, t)
	outsideKey := testAccObjectKey(rName, "outside-before")
	matchedKey := testAccObjectKey(rName, "matched-after")
	testAccPutObjectForRecover(ctx, t, bucket, outsideKey)
	testAccSoftDeleteObjectForRecover(ctx, t, bucket, outsideKey)
	time.Sleep(2 * time.Second)
	after := time.Now().UnixMilli()
	time.Sleep(2 * time.Second)
	testAccPutObjectForRecover(ctx, t, bucket, matchedKey)
	testAccSoftDeleteObjectForRecover(ctx, t, bucket, matchedKey)
	resourceName := "ctyun_oos_object_recover.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectRecoverConfig_timeWindowWithSoftDelete(bucket, &after, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "soft_delete_after", fmt.Sprintf("%d", after)),
					resource.TestCheckResourceAttr(resourceName, "recovered_keys.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "errors.#", "0"),
					testAccCheckRecoveredKeysContains(resourceName, matchedKey),
					testAccCheckObjectRecoverReadable(ctx, t, bucket, matchedKey),
					testAccCheckObjectRecoverNotReadable(ctx, t, bucket, outsideKey),
				),
			},
			{
				Config: testAccObjectRecoverConfig_recoverWithBucket(bucket, outsideKey),
				Check:  resource.TestCheckResourceAttr(resourceName, "errors.#", "0"),
			},
		},
	})
}

// TC-S3-OBJECT-RECOVER-03 Recover By Time Window - before only
// TestAccS3ObjectRecover_RecoverByTimeWindow_before restores only objects soft-deleted at or before the selector.
func TestAccS3ObjectRecover_RecoverByTimeWindow_before(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-object-recover")
	bucket := acctest.OOSSharedObjectRecoverBeforeBucket(ctx, t)
	matchedKey := testAccObjectKey(rName, "matched-before")
	outsideKey := testAccObjectKey(rName, "outside-after")
	testAccPutObjectForRecover(ctx, t, bucket, matchedKey)
	testAccSoftDeleteObjectForRecover(ctx, t, bucket, matchedKey)
	time.Sleep(2 * time.Second)
	before := time.Now().UnixMilli()
	time.Sleep(2 * time.Second)
	testAccPutObjectForRecover(ctx, t, bucket, outsideKey)
	testAccSoftDeleteObjectForRecover(ctx, t, bucket, outsideKey)
	resourceName := "ctyun_oos_object_recover.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectRecoverConfig_timeWindowWithSoftDelete(bucket, nil, &before),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "soft_delete_before", fmt.Sprintf("%d", before)),
					resource.TestCheckResourceAttr(resourceName, "recovered_keys.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "errors.#", "0"),
					testAccCheckRecoveredKeysContains(resourceName, matchedKey),
					testAccCheckObjectRecoverReadable(ctx, t, bucket, matchedKey),
					testAccCheckObjectRecoverNotReadable(ctx, t, bucket, outsideKey),
				),
			},
			{
				Config: testAccObjectRecoverConfig_recoverWithBucket(bucket, outsideKey),
				Check:  resource.TestCheckResourceAttr(resourceName, "errors.#", "0"),
			},
		},
	})
}

// TC-S3-OBJECT-RECOVER-03 Recover By Time Window - bounded range
// TestAccS3ObjectRecover_RecoverByTimeWindow_range restores only objects soft-deleted inside the bounded window.
func TestAccS3ObjectRecover_RecoverByTimeWindow_range(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-object-recover")
	bucket := acctest.OOSSharedObjectRecoverRangeBucket(ctx, t)
	beforeKey := testAccObjectKey(rName, "outside-before")
	matchedKey := testAccObjectKey(rName, "matched")
	afterKey := testAccObjectKey(rName, "outside-after")
	testAccPutObjectForRecover(ctx, t, bucket, beforeKey)
	testAccSoftDeleteObjectForRecover(ctx, t, bucket, beforeKey)
	time.Sleep(2 * time.Second)
	after := time.Now().UnixMilli()
	time.Sleep(2 * time.Second)
	testAccPutObjectForRecover(ctx, t, bucket, matchedKey)
	testAccSoftDeleteObjectForRecover(ctx, t, bucket, matchedKey)
	before := time.Now().UnixMilli() + 1000
	time.Sleep(2 * time.Second)
	testAccPutObjectForRecover(ctx, t, bucket, afterKey)
	testAccSoftDeleteObjectForRecover(ctx, t, bucket, afterKey)
	resourceName := "ctyun_oos_object_recover.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectRecoverConfig_timeWindowWithSoftDelete(bucket, &after, &before),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "soft_delete_after", fmt.Sprintf("%d", after)),
					resource.TestCheckResourceAttr(resourceName, "soft_delete_before", fmt.Sprintf("%d", before)),
					resource.TestCheckResourceAttr(resourceName, "recovered_keys.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "errors.#", "0"),
					testAccCheckRecoveredKeysContains(resourceName, matchedKey),
					testAccCheckObjectRecoverReadable(ctx, t, bucket, matchedKey),
					testAccCheckObjectRecoverNotReadable(ctx, t, bucket, beforeKey),
					testAccCheckObjectRecoverNotReadable(ctx, t, bucket, afterKey),
				),
			},
			{
				Config: testAccObjectRecoverConfig_recoverWithBucket(bucket, beforeKey, afterKey),
				Check:  resource.TestCheckResourceAttr(resourceName, "errors.#", "0"),
			},
		},
	})
}

// TC-S3-OBJECT-RECOVER-04 Recover Missing Object Negative
// TestAccS3ObjectRecover_RecoverMissingObjectNegative_missingKey verifies that recovering a non-existent key makes the provider return a determinable recover error instead of treating the operation as an idempotent success.
func TestAccS3ObjectRecover_RecoverMissingObjectNegative_missingKey(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-object-recover")
	bucket := acctest.OOSSharedObjectsSoftDeleteBucket(ctx, t)
	missingKey := testAccObjectKey(rName, "missing-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccObjectRecoverConfig_singleRecover(bucket, missingKey),
				ExpectError: regexp.MustCompile(fmt.Sprintf(`recovering OOS Objects in Bucket \(%s\): .*%s`, regexp.QuoteMeta(bucket), regexp.QuoteMeta(missingKey))),
			},
		},
	})
}

// TC-S3-OBJECT-RECOVER-05 Repeat Recover
// TestAccS3ObjectRecover_RepeatRecover_basic verifies that recovering an already-recovered object returns a stable error, and that subsequent refreshes leave no dirty state for the recover resource.
func TestAccS3ObjectRecover_RepeatRecover_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, "tf-test-object-recover")
	bucket := acctest.OOSSharedObjectsSoftDeleteBucket(ctx, t)
	key := testAccObjectKey(rName, "recover-key")
	firstResourceName := "ctyun_oos_object_recover.first"
	secondResourceName := "ctyun_oos_object_recover.second"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					testAccPutObjectForRecover(ctx, t, bucket, key)
					testAccSoftDeleteObjectForRecover(ctx, t, bucket, key)
				},
				Config: testAccObjectRecoverConfig_namedRecover("first", bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(firstResourceName, "recovered_keys.#", "1"),
					resource.TestCheckResourceAttr(firstResourceName, "recovered_keys.0", key),
					resource.TestCheckResourceAttr(firstResourceName, "errors.#", "0"),
					testAccCheckObjectRecoverReadable(ctx, t, bucket, key),
				),
			},
			{
				Config:      testAccObjectRecoverConfig_namedRecover("second", bucket, key),
				ExpectError: regexp.MustCompile(fmt.Sprintf(`recovering OOS Objects in Bucket \(%s\): .*%s`, regexp.QuoteMeta(bucket), regexp.QuoteMeta(key))),
			},
			{
				Config: "terraform {}",
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckResourceNotInState(secondResourceName),
					testAccCheckObjectRecoverReadable(ctx, t, bucket, key),
				),
			},
		},
	})
}

// TC-S3-OBJECT-RECOVER-06 Bucket And Mode Validation
// TestAccS3ObjectRecover_BucketAndModeValidation_missingBucket verifies that bucket is a required parameter.
func TestAccS3ObjectRecover_BucketAndModeValidation_missingBucket(t *testing.T) {
	testAccObjectRecoverValidation(t, testAccObjectRecoverConfig_validation(`
  recover {
    key = "recover-key"
  }
`), `(?i)(missing required argument|required argument.*bucket|bucket.*required)`)
}

// TC-S3-OBJECT-RECOVER-06 Bucket And Mode Validation
// TestAccS3ObjectRecover_BucketAndModeValidation_emptyRecover verifies that recover cannot be an empty list.
func TestAccS3ObjectRecover_BucketAndModeValidation_emptyRecover(t *testing.T) {
	testAccObjectRecoverValidation(t, testAccObjectRecoverConfig_validation(`
  bucket = "tf-test-object-recover"
  dynamic "recover" {
    for_each = []
    content {
      key = "recover-key"
    }
  }
`), `(?is)"recover": one of.*must be\s+specified`)
}

// TC-S3-OBJECT-RECOVER-06 Bucket And Mode Validation
// TestAccS3ObjectRecover_BucketAndModeValidation_recoverWithTimeWindow verifies that recover mode cannot be configured together with time window mode.
func TestAccS3ObjectRecover_BucketAndModeValidation_recoverWithTimeWindow(t *testing.T) {
	testAccObjectRecoverValidation(t, testAccObjectRecoverConfig_validation(`
  bucket            = "tf-test-object-recover"
  recover {
    key = "recover-key"
  }
  soft_delete_after = 1
`), `(?i)(recover.*conflicts.*soft_delete_after|soft_delete_after.*conflicts.*recover)`)
}

// TC-S3-OBJECT-RECOVER-06 Bucket And Mode Validation
// TestAccS3ObjectRecover_BucketAndModeValidation_missingMode verifies that either recover or at least one time window boundary must be selected.
func TestAccS3ObjectRecover_BucketAndModeValidation_missingMode(t *testing.T) {
	testAccObjectRecoverValidation(t, testAccObjectRecoverConfig_validation(`
  bucket = "tf-test-object-recover"
`), `(?i)one of`)
}

func testAccObjectRecoverValidation(t *testing.T, config, errorRegexp string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      config,
			ExpectError: regexp.MustCompile(errorRegexp),
		}},
	})
}
func testAccPutObjectForRecover(ctx context.Context, t *testing.T, bucket, key string) {
	t.Helper()

	_, err := acctest.ProviderMeta(ctx, t).S3Client(ctx).PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader("recover me"),
	}, testAccOOSPutObjectOptions)
	if err != nil {
		t.Fatalf("putting object for recovery (%s/%s): %s", bucket, key, err)
	}
}

func testAccSoftDeleteObjectForRecover(ctx context.Context, t *testing.T, bucket, key string) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
	_, err := conn.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("soft-deleting object for recovery (%s/%s): %s", bucket, key, err)
	}

	_, err = tfresource.RetryUntilNotFound(ctx, tfs3.BucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return tfs3.FindObjectByBucketAndKey(ctx, conn, bucket, key, "", "")
	})
	if err != nil {
		t.Fatalf("waiting for soft-deleted object (%s/%s): %s", bucket, key, err)
	}
}

func testAccCheckObjectRecoverReadable(ctx context.Context, t *testing.T, bucket, key string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		_, err := tfresource.RetryWhenNotFound(ctx, tfs3.BucketPropagationTimeout, func(ctx context.Context) (*s3.HeadObjectOutput, error) {
			return tfs3.FindObjectByBucketAndKey(ctx, conn, bucket, key, "", "")
		})
		if err != nil {
			return fmt.Errorf("waiting for recovered object (%s/%s): %w", bucket, key, err)
		}

		output, err := conn.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("reading recovered object (%s/%s): %w", bucket, key, err)
		}
		defer output.Body.Close()

		body, err := io.ReadAll(output.Body)
		if err != nil {
			return fmt.Errorf("reading recovered object body (%s/%s): %w", bucket, key, err)
		}
		if got, want := string(body), "recover me"; got != want {
			return fmt.Errorf("recovered object body (%s/%s): got %q, want %q", bucket, key, got, want)
		}

		return nil
	}
}

func testAccCheckObjectRecoverNotReadable(ctx context.Context, t *testing.T, bucket, key string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		_, err := tfresource.RetryUntilNotFound(ctx, tfs3.BucketPropagationTimeout, func(ctx context.Context) (any, error) {
			return tfs3.FindObjectByBucketAndKey(ctx, conn, bucket, key, "", "")
		})
		if err != nil {
			return fmt.Errorf("waiting for out-of-window object (%s/%s) to remain soft-deleted: %w", bucket, key, err)
		}

		return nil
	}
}

func testAccObjectRecoverConfig_recoverWithBucket(bucket string, keys ...string) string {
	return testAccObjectRecoverConfig_recover(bucket, keys...)
}
func testAccObjectRecoverConfig_timeWindowWithSoftDelete(bucket string, after, before *int64) string {
	var selector strings.Builder
	if after != nil {
		fmt.Fprintf(&selector, "  soft_delete_after  = %d\n", *after)
	}
	if before != nil {
		fmt.Fprintf(&selector, "  soft_delete_before = %d\n", *before)
	}

	return fmt.Sprintf(`
resource "ctyun_oos_object_recover" "test" {
  bucket = %[1]q

%[2]s
}
`, bucket, selector.String())
}
func testAccObjectRecoverConfig_validation(body string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object_recover" "test" {
%s
}
`, body)
}
func testAccObjectRecoverConfig_singleRecover(bucket, key string) string {
	return testAccObjectRecoverConfig_recoverWithSoftDelete(bucket, key)
}

func testAccObjectRecoverConfig_recoverWithSoftDelete(bucket string, recoverKeys ...string) string {
	return testAccObjectRecoverConfig_namedRecover("test", bucket, recoverKeys...)
}

func testAccObjectRecoverConfig_namedRecover(name, bucket string, recoverKeys ...string) string {
	recoverBlocks := make([]string, 0, len(recoverKeys))
	for _, key := range recoverKeys {
		recoverBlocks = append(recoverBlocks, fmt.Sprintf(`
  recover {
    key = %q
  }`, key))
	}

	return fmt.Sprintf(`
resource "ctyun_oos_object_recover" %q {
  bucket = %q
%s
}
`, name, bucket, strings.Join(recoverBlocks, ""))
}

func testAccObjectRecoverConfig_namedRecoverWithEncodingType(name, bucket, encodingType string, recoverKeys ...string) string {
	recoverBlocks := make([]string, 0, len(recoverKeys))
	for _, key := range recoverKeys {
		recoverBlocks = append(recoverBlocks, fmt.Sprintf(`
  recover {
    key = %q
  }`, key))
	}

	encodingTypeConfig := ""
	if encodingType != "" {
		encodingTypeConfig = fmt.Sprintf("  encoding_type = %q\n", encodingType)
	}

	return fmt.Sprintf(`
resource "ctyun_oos_object_recover" %q {
  bucket = %q
%s%s
}
`, name, bucket, encodingTypeConfig, strings.Join(recoverBlocks, ""))
}
