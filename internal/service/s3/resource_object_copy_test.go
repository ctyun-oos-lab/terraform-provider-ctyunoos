// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-S3-OBJECT-COPY-01
const testAccObjectCopyMetadataMaxSize = 2048

// TC-S3-OBJECT-COPY-01 Cross Bucket Copy
// TestAccS3ObjectCopy_CrossBucketCopy_basic verifies cross-bucket copy, the target object content, and the basic state fields
func TestAccS3ObjectCopy_CrossBucketCopy_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s.txt", rName)
	targetKey := fmt.Sprintf("copied-%s.txt", rName)
	resourceName := "ctyun_oos_object_copy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{{
			Config: testAccObjectCopyConfig_basic(sourceBucket, targetBucket, sourceKey, targetKey),
			Check: resource.ComposeAggregateTestCheckFunc(
				testAccCheckObjectCopyExists(ctx, t, resourceName),
				testAccCheckObjectCopyContent(ctx, t, resourceName, "some_bucket_content"),
				testAccCheckObjectCopyAllOutputs(ctx, t, resourceName),
				resource.TestCheckResourceAttr(resourceName, names.AttrID, targetKey),
				resource.TestCheckResourceAttr(resourceName, names.AttrARN, fmt.Sprintf("arn:ctyun:oos:::%s/%s", targetBucket, targetKey)),
				resource.TestCheckResourceAttr(resourceName, names.AttrSource, fmt.Sprintf("%s/%s", sourceBucket, sourceKey)),
				resource.TestCheckResourceAttr(resourceName, names.AttrBucket, targetBucket),
				resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey),
			),
		}},
	})
}

// TC-S3-OBJECT-COPY-02 Source And Destination Validation
// TestAccS3ObjectCopy_SourceAndDestinationValidation_invalidSourceBucket verifies that an illegal source bucket is rejected
func TestAccS3ObjectCopy_SourceAndDestinationValidation_invalidSourceBucket(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_validation(targetBucket, "target.txt", "invalid_bucket/source.txt"), `(?i)(InvalidBucketName|invalid bucket|bucket[^\n]*invalid)`)
}

// TC-S3-OBJECT-COPY-02 Source And Destination Validation
// TestAccS3ObjectCopy_SourceAndDestinationValidation_sourceBucketNotFound verifies that a non-existent source bucket returns a clear error
func TestAccS3ObjectCopy_SourceAndDestinationValidation_sourceBucketNotFound(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_validation(targetBucket, "target.txt", fmt.Sprintf("%s/source.txt", rName)), `(?i)(NoSuchBucket|not found|non-existent)`)
}

// TC-S3-OBJECT-COPY-02 Source And Destination Validation
// TestAccS3ObjectCopy_SourceAndDestinationValidation_emptySourceBucket verifies that an empty source bucket is rejected
func TestAccS3ObjectCopy_SourceAndDestinationValidation_emptySourceBucket(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_validation(targetBucket, "target.txt", "/source.txt"), `(?i)(InvalidBucketName|InvalidArgument|invalid bucket|bucket[^\n]*empty)`)
}

// TC-S3-OBJECT-COPY-02 Source And Destination Validation
// TestAccS3ObjectCopy_SourceAndDestinationValidation_invalidTargetBucket verifies that an illegal target bucket is rejected
func TestAccS3ObjectCopy_SourceAndDestinationValidation_invalidTargetBucket(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_basic(sourceBucket, "invalid_bucket", fmt.Sprintf("source-%s", rName), "target.txt"), `(?i)(InvalidBucketName|invalid bucket|invalid value for bucket|bucket[^\n]*invalid)`)
}

// TC-S3-OBJECT-COPY-02 Source And Destination Validation
// TestAccS3ObjectCopy_SourceAndDestinationValidation_targetBucketNotFound verifies that a non-existent target bucket returns a clear error
func TestAccS3ObjectCopy_SourceAndDestinationValidation_targetBucketNotFound(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_basic(sourceBucket, rName, fmt.Sprintf("source-%s", rName), "target.txt"), `(?i)(NoSuchBucket|not found|non-existent)`)
}

// TC-S3-OBJECT-COPY-02 Source And Destination Validation
// TestAccS3ObjectCopy_SourceAndDestinationValidation_emptyTargetBucket verifies that an empty target bucket is rejected
func TestAccS3ObjectCopy_SourceAndDestinationValidation_emptyTargetBucket(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_basic(sourceBucket, "", fmt.Sprintf("source-%s", rName), "target.txt"), `(?i)(expected length of bucket.*\(3 - 63\)|bucket string length|bucket.*empty|must not be empty)`)
}

// TC-S3-OBJECT-COPY-02 Source And Destination Validation
// TestAccS3ObjectCopy_SourceAndDestinationValidation_emptySourceKey verifies that an empty source key is rejected
func TestAccS3ObjectCopy_SourceAndDestinationValidation_emptySourceKey(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_validation(targetBucket, "target.txt", fmt.Sprintf("%s/", sourceBucket)), `(?i)(Invalid.*Key|NoSuchKey|key[^\n]*empty|invalid.*source)`)
}

// TC-S3-OBJECT-COPY-02 Source And Destination Validation
// TestAccS3ObjectCopy_SourceAndDestinationValidation_emptyTargetKey verifies that an empty target key is rejected
func TestAccS3ObjectCopy_SourceAndDestinationValidation_emptyTargetKey(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_basic(sourceBucket, targetBucket, fmt.Sprintf("source-%s", rName), ""), `(?i)(Invalid.*Key|NoSuchKey|key[^\n]*empty)`)
}

// TC-S3-OBJECT-COPY-03 Same Bucket Different Key
// TestAccS3ObjectCopy_SameBucketDifferentKey_basic verifies that copying within the same bucket to a different key succeeds and the source object is not overwritten
func TestAccS3ObjectCopy_SameBucketDifferentKey_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s.txt", rName)
	targetKey := fmt.Sprintf("copied-%s.txt", rName)
	content := "some_bucket_content"
	resourceName := "ctyun_oos_object_copy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{{
			Config: testAccObjectCopyConfig_basic(bucket, bucket, sourceKey, targetKey),
			Check: resource.ComposeAggregateTestCheckFunc(
				testAccCheckObjectCopyExists(ctx, t, resourceName),
				testAccCheckObjectCopyContent(ctx, t, resourceName, content),
				testAccCheckObjectCopyRemoteContent(ctx, t, bucket, sourceKey, content),
				resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucket),
				resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey),
				resource.TestCheckResourceAttr(resourceName, names.AttrSource, fmt.Sprintf("%s/%s", bucket, sourceKey)),
			),
		}},
	})
}

// TC-S3-OBJECT-COPY-03 Same Bucket Different Key
// TestAccS3ObjectCopy_SameBucketDifferentKey_crossBucket verifies that the standalone cross-bucket copy path succeeds and the source object is not overwritten
func TestAccS3ObjectCopy_SameBucketDifferentKey_crossBucket(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s.txt", rName)
	targetKey := fmt.Sprintf("copied-%s.txt", rName)
	content := "some_bucket_content"
	resourceName := "ctyun_oos_object_copy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{{
			Config: testAccObjectCopyConfig_basic(sourceBucket, targetBucket, sourceKey, targetKey),
			Check: resource.ComposeAggregateTestCheckFunc(
				testAccCheckObjectCopyExists(ctx, t, resourceName),
				testAccCheckObjectCopyContent(ctx, t, resourceName, content),
				testAccCheckObjectCopyRemoteContent(ctx, t, sourceBucket, sourceKey, content),
				resource.TestCheckResourceAttr(resourceName, names.AttrBucket, targetBucket),
				resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey),
				resource.TestCheckResourceAttr(resourceName, names.AttrSource, fmt.Sprintf("%s/%s", sourceBucket, sourceKey)),
			),
		}},
	})
}

// TC-S3-OBJECT-COPY-04 Special Key And Length Boundaries
// TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_specialSourceKey verifies that a special-character key that CopySource can pass stably can be copied
func TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_specialSourceKey(t *testing.T) {
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	testAccObjectCopyKeyBoundarySuccess(t, fmt.Sprintf("source-%s-_.!()", rName), fmt.Sprintf("target-%s.txt", rName))
}

// TC-S3-OBJECT-COPY-04 Special Key And Length Boundaries
// TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_specialTargetKey verifies that a special-character target key can be copied
func TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_specialTargetKey(t *testing.T) {
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	testAccObjectCopyKeyBoundarySuccess(t, fmt.Sprintf("source-%s.txt", rName), fmt.Sprintf("target-%s-_.!()'@&=+$,;", rName))
}

// TC-S3-OBJECT-COPY-04 Special Key And Length Boundaries
// TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_maxLengthSourceKey verifies that a 1024-byte key can be copied
func TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_maxLengthSourceKey(t *testing.T) {
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	testAccObjectCopyKeyBoundarySuccess(t, testAccObjectCopyMaxLengthKey(rName), fmt.Sprintf("target-%s.txt", rName))
}

// TC-S3-OBJECT-COPY-04 Special Key And Length Boundaries
// TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_maxLengthTargetKey verifies that a 1024-byte target key can be copied
func TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_maxLengthTargetKey(t *testing.T) {
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	testAccObjectCopyKeyBoundarySuccess(t, fmt.Sprintf("source-%s.txt", rName), testAccObjectCopyMaxLengthKey(rName))
}

// TC-S3-OBJECT-COPY-04 Special Key And Length Boundaries
// TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_overlongSourceKey verifies that copying a source key exceeding 1024 bytes fails; OOS returns NoSuchKey
func TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_overlongSourceKey(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_validation(targetBucket, "target.txt", fmt.Sprintf("%s/%s", sourceBucket, strings.Repeat("a", 1025))), `(?i)(InvalidObjectName|NoSuchKey)`)
}

// TC-S3-OBJECT-COPY-04 Special Key And Length Boundaries
// TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_overlongTargetKey verifies that a target key exceeding 1024 bytes is rejected
func TestAccS3ObjectCopy_SpecialKeyAndLengthBoundaries_overlongTargetKey(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_basic(sourceBucket, targetBucket, fmt.Sprintf("source-%s", rName), strings.Repeat("a", 1025)), `(?i)InvalidObjectName`)
}

// TC-S3-OBJECT-COPY-05 Storage Class
// TestAccS3ObjectCopy_StorageClass_standard verifies that storage_class reads back correctly after being explicitly set to STANDARD.
func TestAccS3ObjectCopy_StorageClass_standard(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_object_copy.test"
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName)
	targetKey := fmt.Sprintf("target-%s", rName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{{
			Config: testAccObjectCopyConfig_storageClass(sourceBucket, targetBucket, sourceKey, targetKey, "STANDARD"),
			Check: resource.ComposeAggregateTestCheckFunc(
				testAccCheckObjectCopyExists(ctx, t, resourceName),
				testAccCheckObjectCopyContent(ctx, t, resourceName, "some_bucket_content"),
				resource.TestCheckResourceAttr(resourceName, names.AttrStorageClass, "STANDARD"),
				resource.TestCheckResourceAttr(resourceName, names.AttrBucket, targetBucket),
				resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey),
			),
		}},
	})
}

// TC-S3-OBJECT-COPY-05 Storage Class
// TestAccS3ObjectCopy_StorageClass_standardIA verifies that storage_class is set during copy and reads back correctly
func TestAccS3ObjectCopy_StorageClass_standardIA(t *testing.T) {
	t.Skip("temporarily skipped")

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_object_copy.test"
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName)
	targetKey := fmt.Sprintf("target-%s", rName)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectCopyConfig_base(sourceBucket, targetBucket, sourceKey),
				Check:  testAccCheckObjectContentEventually(ctx, t, "ctyun_oos_object.source", "some_bucket_content"),
			},
			{
				Config: testAccObjectCopyConfig_storageClass(sourceBucket, targetBucket, sourceKey, targetKey, "STANDARD_IA"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectCopyExists(ctx, t, resourceName),
					testAccCheckObjectContentEventually(ctx, t, resourceName, "some_bucket_content"),
					resource.TestCheckResourceAttr(resourceName, names.AttrStorageClass, "STANDARD_IA"),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, targetBucket),
					resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey),
					resource.TestCheckResourceAttr(resourceName, names.AttrSource, fmt.Sprintf("%s/%s", sourceBucket, sourceKey)),
				),
			},
		},
	})
}

// TC-S3-OBJECT-COPY-05 Storage Class
// TestAccS3ObjectCopy_StorageClass_invalid verifies that an illegal storage_class is rejected
func TestAccS3ObjectCopy_StorageClass_invalid(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName)
	targetKey := fmt.Sprintf("target-%s", rName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccObjectCopyConfig_storageClass(sourceBucket, targetBucket, sourceKey, targetKey, "INVALID"),
			ExpectError: regexache.MustCompile(`(?i)expected.*STANDARD`),
		}},
	})
}

// TC-S3-OBJECT-COPY-06 Metadata Replace
// TestAccS3ObjectCopy_MetadataReplace_replace verifies that after metadata is replaced, the target object reads back the new values
func TestAccS3ObjectCopy_MetadataReplace_replace(t *testing.T) {
	testAccObjectCopyMetadataReplaceValid(t, "key1", "replacement-value", "1")
}

// TC-S3-OBJECT-COPY-06 Metadata Replace
// TestAccS3ObjectCopy_MetadataReplace_empty verifies that metadata state stays empty when unset
func TestAccS3ObjectCopy_MetadataReplace_empty(t *testing.T) {
	testAccObjectCopyMetadataReplaceValid(t, "", "", "0")
}

// TC-S3-OBJECT-COPY-06 Metadata Replace
// TestAccS3ObjectCopy_MetadataReplace_maxLength verifies that 2048 bytes of metadata can be copied and read back
func TestAccS3ObjectCopy_MetadataReplace_maxLength(t *testing.T) {
	testAccObjectCopyMetadataReplaceValid(t, "metakey", strings.Repeat("a", testAccObjectCopyMetadataMaxSize), "1")
}

// TC-S3-OBJECT-COPY-06 Metadata Replace
// TestAccS3ObjectCopy_MetadataReplace_overlong verifies that metadata exceeding 2048 bytes is rejected
func TestAccS3ObjectCopy_MetadataReplace_overlong(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName)
	targetKey := fmt.Sprintf("target-%s", rName)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_metadataCustom(sourceBucket, targetBucket, sourceKey, targetKey, "metakey", strings.Repeat("a", testAccObjectCopyMetadataMaxSize+1)), `(?i)InvalidArgument:\s*The metadata size should be less than 2K`)
}

// TC-S3-OBJECT-COPY-06 Metadata Replace
// TestAccS3ObjectCopy_MetadataReplace_keyNonLowercase verifies that a non-lowercase metadata key is rejected
func TestAccS3ObjectCopy_MetadataReplace_keyNonLowercase(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName)
	targetKey := fmt.Sprintf("target-%s", rName)

	testAccObjectCopyValidation(t, testAccObjectCopyConfig_metadataCustom(sourceBucket, targetBucket, sourceKey, targetKey, "Key1", "value1"), `Metadata must be lowercase only`)
}

// TC-S3-OBJECT-COPY-06 Metadata Replace
// TestAccS3ObjectCopy_MetadataCopy_copy verifies that with metadata_directive=COPY the target object keeps the source object's metadata.
func TestAccS3ObjectCopy_MetadataCopy_copy(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_object_copy.test"
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName)
	targetKey := fmt.Sprintf("target-%s", rName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{{
			Config: testAccObjectCopyConfig_metadataCopy(sourceBucket, targetBucket, sourceKey, targetKey),
			Check: resource.ComposeAggregateTestCheckFunc(
				testAccCheckObjectCopyExists(ctx, t, resourceName),
				resource.TestCheckResourceAttr(resourceName, "metadata_directive", "COPY"),
				resource.TestCheckResourceAttr(resourceName, "metadata.%", "1"),
				resource.TestCheckResourceAttr(resourceName, "metadata.sourcekey", "source-value"),
				testAccCheckObjectCopyMetadata(ctx, t, resourceName, map[string]string{"sourcekey": "source-value"}),
			),
		}},
	})
}

// TC-S3-OBJECT-COPY-08 Conditional Copy
// TestAccS3ObjectCopy_ConditionalCopy_copyIfMatchSatisfied verifies that the copy succeeds when copy_if_match matches the ETag
func TestAccS3ObjectCopy_ConditionalCopy_copyIfMatchSatisfied(t *testing.T) {
	testAccObjectCopyConditional(t, "copy_if_match", `format("\"%s\"", ctyun_oos_object.source.etag)`, false)
}

// TC-S3-OBJECT-COPY-08 Conditional Copy
// TestAccS3ObjectCopy_ConditionalCopy_copyIfMatchFailed verifies that apply fails when copy_if_match does not match
func TestAccS3ObjectCopy_ConditionalCopy_copyIfMatchFailed(t *testing.T) {
	testAccObjectCopyConditional(t, "copy_if_match", fmt.Sprintf("%q", "\"00000000000000000000000000000000\""), true)
}

// TC-S3-OBJECT-COPY-08 Conditional Copy
// TestAccS3ObjectCopy_ConditionalCopy_copyIfNoneMatchSatisfied verifies that the copy succeeds when copy_if_none_match does not match the source ETag
func TestAccS3ObjectCopy_ConditionalCopy_copyIfNoneMatchSatisfied(t *testing.T) {
	testAccObjectCopyConditional(t, "copy_if_none_match", fmt.Sprintf("%q", "\"00000000000000000000000000000000\""), false)
}

// TC-S3-OBJECT-COPY-08 Conditional Copy
// TestAccS3ObjectCopy_ConditionalCopy_copyIfNoneMatchFailed verifies that apply fails when copy_if_none_match matches the ETag
func TestAccS3ObjectCopy_ConditionalCopy_copyIfNoneMatchFailed(t *testing.T) {
	testAccObjectCopyConditional(t, "copy_if_none_match", `format("\"%s\"", ctyun_oos_object.source.etag)`, true)
}

// TC-S3-OBJECT-COPY-08 Conditional Copy
// ctyun_oos_object does not export last_modified, so stable past/future RFC3339 values bracket its apply-time creation without plan drift.
// TestAccS3ObjectCopy_ConditionalCopy_copyIfModifiedSinceSatisfied verifies that the copy succeeds when copy_if_modified_since is earlier than the source object
func TestAccS3ObjectCopy_ConditionalCopy_copyIfModifiedSinceSatisfied(t *testing.T) {
	testAccObjectCopyConditional(t, "copy_if_modified_since", `"2000-01-01T00:00:00Z"`, false)
}

// TC-S3-OBJECT-COPY-08 Conditional Copy
// TestAccS3ObjectCopy_ConditionalCopy_copyIfModifiedSinceFailed verifies that apply fails when copy_if_modified_since is later than the source object
func TestAccS3ObjectCopy_ConditionalCopy_copyIfModifiedSinceFailed(t *testing.T) {
	testAccObjectCopyConditional(t, "copy_if_modified_since", `"2100-01-01T00:00:00Z"`, true)
}

// TC-S3-OBJECT-COPY-08 Conditional Copy
// TestAccS3ObjectCopy_ConditionalCopy_copyIfUnmodifiedSinceSatisfied verifies that the copy succeeds when copy_if_unmodified_since is later than the source object
func TestAccS3ObjectCopy_ConditionalCopy_copyIfUnmodifiedSinceSatisfied(t *testing.T) {
	testAccObjectCopyConditional(t, "copy_if_unmodified_since", `"2100-01-01T00:00:00Z"`, false)
}

// TC-S3-OBJECT-COPY-08 Conditional Copy
// TestAccS3ObjectCopy_ConditionalCopy_copyIfUnmodifiedSinceFailed verifies that apply fails when copy_if_unmodified_since is earlier than the source object
func TestAccS3ObjectCopy_ConditionalCopy_copyIfUnmodifiedSinceFailed(t *testing.T) {
	testAccObjectCopyConditional(t, "copy_if_unmodified_since", `"2000-01-01T00:00:00Z"`, true)
}

// TC-S3-OBJECT-COPY-14 Update Validation
// TestAccS3ObjectCopy_UpdateValidation_cacheControl verifies that updating cache_control triggers a re-copy and reads back the new value.
func TestAccS3ObjectCopy_UpdateValidation_cacheControl(t *testing.T) {
	testAccObjectCopyUpdateHeader(t, "cache_control", "max-age=3600")
}

// TC-S3-OBJECT-COPY-14 Update Validation
// TestAccS3ObjectCopy_UpdateValidation_contentDisposition verifies that updating content_disposition triggers a re-copy and reads back the new value.
func TestAccS3ObjectCopy_UpdateValidation_contentDisposition(t *testing.T) {
	testAccObjectCopyUpdateHeader(t, "content_disposition", `attachment; filename="copy.txt"`)
}

// TC-S3-OBJECT-COPY-14 Update Validation
// TestAccS3ObjectCopy_UpdateValidation_contentEncoding verifies that updating content_encoding triggers a re-copy and reads back the new value.
func TestAccS3ObjectCopy_UpdateValidation_contentEncoding(t *testing.T) {
	testAccObjectCopyUpdateHeader(t, "content_encoding", "gzip")
}

// TC-S3-OBJECT-COPY-14 Update Validation
// TestAccS3ObjectCopy_UpdateValidation_contentType verifies that updating content_type triggers a re-copy and reads back the new value.
func TestAccS3ObjectCopy_UpdateValidation_contentType(t *testing.T) {
	testAccObjectCopyUpdateHeader(t, names.AttrContentType, "text/plain")
}

// TC-S3-OBJECT-COPY-09 Overwrite Existing Target
// TestAccS3ObjectCopy_OverwriteExistingTarget_basic verifies that copying overwrites the existing target object and the plan is stable after refresh
func TestAccS3ObjectCopy_OverwriteExistingTarget_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_object_copy.test"
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName)
	targetKey := fmt.Sprintf("target-%s", rName)
	config := testAccObjectCopyConfig_overwriteTarget(sourceBucket, targetBucket, sourceKey, targetKey)

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectCopyExists(ctx, t, resourceName),
		testAccCheckObjectCopyContent(ctx, t, resourceName, "some_bucket_content"),
		resource.TestCheckResourceAttr(resourceName, names.AttrBucket, targetBucket),
		resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey),
		resource.TestCheckResourceAttr(resourceName, names.AttrSource, fmt.Sprintf("%s/%s", sourceBucket, sourceKey)),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  checks,
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-OBJECT-COPY-10 Cross Account Negative
// TestAccS3ObjectCopy_CrossAccountNegative_readSource verifies that the current account cannot copy another account's source object; OOS may hide the object's existence behind NoSuchKey
func TestAccS3ObjectCopy_CrossAccountNegative_readSource(t *testing.T) {
	ctx := acctest.Context(t)
	sourceBucket := acctest.SkipIfEnvVarNotSet(t, "CTYUNOOS_OTHER_ACCOUNT_BUCKET_NAME")
	sourceKey := acctest.SkipIfEnvVarNotSet(t, "CTYUNOOS_OTHER_ACCOUNT_OBJECT_KEY")
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	targetKey := fmt.Sprintf("cross-account-target-%s", acctest.RandomWithPrefix(t, acctest.ResourcePrefix))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccObjectCopyConfig_validation(targetBucket, targetKey, fmt.Sprintf("%s/%s", sourceBucket, sourceKey)),
			ExpectError: regexache.MustCompile(`(?i)(AccessDenied:\s*Access Denied|NoSuchKey)`),
		}},
	})
}

// TC-S3-OBJECT-COPY-11 Destroy Verification
// TestAccS3ObjectCopy_DestroyVerification_basic verifies that the target object is deleted after the copy resource is destroyed
func TestAccS3ObjectCopy_DestroyVerification_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_object_copy.test"
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName)
	targetKey := fmt.Sprintf("destroy-target-%s", rName)
	config := testAccObjectCopyConfig_basic(sourceBucket, targetBucket, sourceKey, targetKey)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectCopyExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, targetBucket),
					resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey),
				),
			},
			{
				Config:  config,
				Destroy: true,
			},
		},
	})
}

// TC-S3-OBJECT-COPY-13 Server Side Encryption
// TestAccS3ObjectCopy_ServerSideEncryption_noBucketEncryption verifies that a copied object does not read back server_side_encryption when bucket encryption is not configured
func TestAccS3ObjectCopy_ServerSideEncryption_noBucketEncryption(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s.txt", rName)
	targetKey := fmt.Sprintf("copied-%s.txt", rName)
	resourceName := "ctyun_oos_object_copy.test"
	config := testAccObjectCopyConfig_basic(bucket, bucket, sourceKey, targetKey)
	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectCopyExists(ctx, t, resourceName),
		resource.TestCheckResourceAttr(resourceName, "server_side_encryption", ""),
		resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucket),
		resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey),
		resource.TestCheckResourceAttr(resourceName, names.AttrSource, fmt.Sprintf("%s/%s", bucket, sourceKey)),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: config, Check: checks},
			{RefreshState: true, Check: checks},
		},
	})
}

// TC-S3-OBJECT-COPY-13 Server Side Encryption
// TestAccS3ObjectCopy_ServerSideEncryption_bucketEncryption verifies that a copied object in an encrypted bucket reads back server_side_encryption
func TestAccS3ObjectCopy_ServerSideEncryption_bucketEncryption(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s.txt", rName)
	targetKey := fmt.Sprintf("copied-%s.txt", rName)
	resourceName := "ctyun_oos_object_copy.test"
	config := testAccObjectCopyConfig_serverSideEncryption(bucket, sourceKey, targetKey)
	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectCopyExists(ctx, t, resourceName),
		resource.TestCheckResourceAttr("ctyun_oos_bucket_server_side_encryption_configuration.test", "rule.#", "1"),
		resource.TestCheckResourceAttr("ctyun_oos_bucket_server_side_encryption_configuration.test", "rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
		resource.TestCheckResourceAttr(resourceName, "server_side_encryption", "AES256"),
		resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucket),
		resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey),
		resource.TestCheckResourceAttr(resourceName, names.AttrSource, fmt.Sprintf("%s/%s", bucket, sourceKey)),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: config, Check: checks},
			{RefreshState: true, Check: checks},
		},
	})
}

// TC-S3-OBJECT-COPY-14 Update Validation
// TestAccS3ObjectCopy_UpdateValidation_targetKey verifies that updating the target key triggers a re-copy and the plan is stable after refresh.
func TestAccS3ObjectCopy_UpdateValidation_targetKey(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_object_copy.test"
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName1)
	targetKey1 := fmt.Sprintf("target-a-%s", rName1)
	targetKey2 := fmt.Sprintf("target-b-%s", rName1)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectCopyConfig_basic(sourceBucket, targetBucket, sourceKey, targetKey1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectCopyExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey1),
				),
			},
			{
				Config: testAccObjectCopyConfig_basic(sourceBucket, targetBucket, sourceKey, targetKey2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectCopyExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey2),
				),
			},
			{
				Config:             testAccObjectCopyConfig_basic(sourceBucket, targetBucket, sourceKey, targetKey2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-OBJECT-COPY-08 Conditional Copy
// TestAccS3ObjectCopy_ConditionalCopy_copyIfModifiedSinceInvalidFormat verifies that a non-RFC3339 conditional time input is rejected.
func TestAccS3ObjectCopy_ConditionalCopy_copyIfModifiedSinceInvalidFormat(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName1)
	targetKey := fmt.Sprintf("target-%s", rName1)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccObjectCopyConfig_copyIfModifiedSince(sourceBucket, targetBucket, sourceKey, targetKey, "not-a-time"),
				ExpectError: regexache.MustCompile(`RFC3339`),
			},
		},
	})
}

func testAccObjectCopyUpdateHeader(t *testing.T, attribute, value string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_object_copy.test"
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName)
	targetKey := fmt.Sprintf("target-%s", rName)
	initialConfig := testAccObjectCopyConfig_updateHeader(sourceBucket, targetBucket, sourceKey, targetKey, "", "")
	updatedConfig := testAccObjectCopyConfig_updateHeader(sourceBucket, targetBucket, sourceKey, targetKey, attribute, value)

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectCopyExists(ctx, t, resourceName),
		testAccCheckObjectCopyContent(ctx, t, resourceName, "some_bucket_content"),
		resource.TestCheckResourceAttr(resourceName, attribute, value),
		testAccCheckObjectCopyHeader(ctx, t, resourceName, attribute, value),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: initialConfig, Check: testAccCheckObjectCopyExists(ctx, t, resourceName)},
			{Config: updatedConfig, Check: checks},
			{Config: updatedConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
		},
	})
}
func testAccObjectCopyConditional(t *testing.T, conditionField, valueExpression string, expectError bool) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_object_copy.test"
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName)
	targetKey := fmt.Sprintf("target-%s", rName)
	config := testAccObjectCopyConfig_conditional(sourceBucket, targetBucket, sourceKey, targetKey, conditionField, valueExpression)

	step := resource.TestStep{Config: config}
	if expectError {
		step.ExpectError = regexache.MustCompile(`(?i)(precondition|condition|match|etag|copy)`)
	} else {
		step.Check = resource.ComposeAggregateTestCheckFunc(
			testAccCheckObjectCopyExists(ctx, t, resourceName),
			testAccCheckObjectCopyContent(ctx, t, resourceName, "some_bucket_content"),
			resource.TestCheckResourceAttrSet(resourceName, "etag"),
			resource.TestCheckResourceAttrSet(resourceName, "last_modified"),
			resource.TestCheckResourceAttr(resourceName, names.AttrBucket, targetBucket),
			resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey),
			resource.TestCheckResourceAttr(resourceName, names.AttrSource, fmt.Sprintf("%s/%s", sourceBucket, sourceKey)),
		)
	}

	testCase := resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps:                    []resource.TestStep{step},
	}
	if !expectError {
		testCase.CheckDestroy = testAccCheckObjectCopyDestroy(ctx, t)
	}
	acctest.ParallelTest(ctx, t, testCase)
}
func testAccObjectCopyMetadataReplaceValid(t *testing.T, metadataKey, metadataValue, metadataCount string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_object_copy.test"
	sourceBucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	targetBucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	sourceKey := fmt.Sprintf("source-%s", rName)
	targetKey := fmt.Sprintf("target-%s", rName)
	config := testAccObjectCopyConfig_metadataCustom(sourceBucket, targetBucket, sourceKey, targetKey, metadataKey, metadataValue)
	if metadataKey == "" {
		config = testAccObjectCopyConfig_metadataEmpty(sourceBucket, targetBucket, sourceKey, targetKey)
	}

	checks := []resource.TestCheckFunc{
		testAccCheckObjectCopyExists(ctx, t, resourceName),
		resource.TestCheckResourceAttr(resourceName, "metadata_directive", "REPLACE"),
		resource.TestCheckResourceAttr(resourceName, "metadata.%", metadataCount),
	}
	if metadataKey != "" {
		checks = append(checks, resource.TestCheckResourceAttr(resourceName, "metadata."+metadataKey, metadataValue))
	}

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{{
			Config: config,
			Check:  resource.ComposeAggregateTestCheckFunc(checks...),
		}},
	})
}
func testAccObjectCopyMaxLengthKey(rName string) string {
	return strings.Repeat("a", 1024-len(rName)-1) + "-" + rName
}
func testAccObjectCopyKeyBoundarySuccess(t *testing.T, sourceKey, targetKey string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	bucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	content := "some_bucket_content"
	resourceName := "ctyun_oos_object_copy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectCopyDestroy(ctx, t),
		Steps: []resource.TestStep{{
			Config: testAccObjectCopyConfig_basic(bucket, bucket, sourceKey, targetKey),
			Check: resource.ComposeAggregateTestCheckFunc(
				testAccCheckObjectCopyExists(ctx, t, resourceName),
				testAccCheckObjectCopyContent(ctx, t, resourceName, content),
				resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucket),
				resource.TestCheckResourceAttr(resourceName, names.AttrKey, targetKey),
				resource.TestCheckResourceAttr(resourceName, names.AttrSource, fmt.Sprintf("%s/%s", bucket, sourceKey)),
			),
		}},
	})
}
func testAccObjectCopyValidation(t *testing.T, config, errorRegexp string) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      config,
			ExpectError: regexache.MustCompile(errorRegexp),
		}},
	})
}
func testAccCheckObjectCopyDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_oos_object_copy" {
				continue
			}

			_, err := tfs3.FindObjectByBucketAndKey(ctx, conn, rs.Primary.Attributes[names.AttrBucket], rs.Primary.Attributes[names.AttrKey], rs.Primary.Attributes["etag"], "")

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("S3 Object %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckObjectCopyExists(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not Found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindObjectByBucketAndKey(ctx, conn, rs.Primary.Attributes[names.AttrBucket], rs.Primary.Attributes[names.AttrKey], rs.Primary.Attributes["etag"], "")
		return err
	}
}

func testAccCheckObjectCopyAllOutputs(ctx context.Context, t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		var headers testAccOOSObjectHeaders
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		output, err := conn.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(rs.Primary.Attributes[names.AttrBucket]),
			Key:    aws.String(rs.Primary.Attributes[names.AttrKey]),
		}, testAccCaptureOOSObjectHeadersMiddleware(&headers))
		if err != nil {
			return err
		}

		checks := map[string]string{
			"cache_control":          aws.ToString(output.CacheControl),
			"content_disposition":    aws.ToString(output.ContentDisposition),
			"content_encoding":       aws.ToString(output.ContentEncoding),
			names.AttrContentType:    aws.ToString(output.ContentType),
			"etag":                   strings.Trim(aws.ToString(output.ETag), `"`),
			"expiration":             aws.ToString(output.Expiration),
			"server_side_encryption": string(output.ServerSideEncryption),
			"data_location":          headers.dataLocation,
			"metadata_location":      headers.metadataLocation,
			"storage_class":          "STANDARD",
		}
		if output.LastModified != nil {
			checks["last_modified"] = output.LastModified.Format(time.RFC3339)
		}
		for key, want := range checks {
			if got := rs.Primary.Attributes[key]; got != want {
				return fmt.Errorf("%s = %q, want %q", key, got, want)
			}
		}

		if got := rs.Primary.Attributes["metadata.%"]; got != strconv.Itoa(len(output.Metadata)) {
			return fmt.Errorf("metadata count = %q, want %d", got, len(output.Metadata))
		}
		for key, want := range output.Metadata {
			if got := rs.Primary.Attributes["metadata."+key]; got != want {
				return fmt.Errorf("metadata.%s = %q, want %q", key, got, want)
			}
		}

		return nil
	}
}
func testAccCheckObjectCopyMetadata(ctx context.Context, t *testing.T, resourceName string, want map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		output, err := conn.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(rs.Primary.Attributes[names.AttrBucket]),
			Key:    aws.String(rs.Primary.Attributes[names.AttrKey]),
		})
		if err != nil {
			return err
		}
		if len(output.Metadata) != len(want) {
			return fmt.Errorf("remote metadata count = %d, want %d", len(output.Metadata), len(want))
		}
		for key, value := range want {
			if got := output.Metadata[key]; got != value {
				return fmt.Errorf("remote metadata.%s = %q, want %q", key, got, value)
			}
		}

		return nil
	}
}
func testAccCheckObjectCopyHeader(ctx context.Context, t *testing.T, resourceName, attribute, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		output, err := conn.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(rs.Primary.Attributes[names.AttrBucket]),
			Key:    aws.String(rs.Primary.Attributes[names.AttrKey]),
		})
		if err != nil {
			return err
		}

		values := map[string]string{
			"cache_control":       aws.ToString(output.CacheControl),
			"content_disposition": aws.ToString(output.ContentDisposition),
			"content_encoding":    aws.ToString(output.ContentEncoding),
			names.AttrContentType: aws.ToString(output.ContentType),
		}
		if got, ok := values[attribute]; !ok {
			return fmt.Errorf("unsupported object copy header attribute %q", attribute)
		} else if got != want {
			return fmt.Errorf("remote %s = %q, want %q", attribute, got, want)
		}

		return nil
	}
}
func testAccCheckObjectCopyRemoteContent(ctx context.Context, t *testing.T, bucket, key, want string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		output, err := conn.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return err
		}
		defer output.Body.Close()

		body, err := io.ReadAll(output.Body)
		if err != nil {
			return err
		}
		if got := string(body); got != want {
			return fmt.Errorf("object content = %q, want %q", got, want)
		}
		return nil
	}
}
func testAccCheckObjectCopyContent(ctx context.Context, t *testing.T, n, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not Found: %s", n)
		}
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		output, err := conn.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(rs.Primary.Attributes[names.AttrBucket]),
			Key:    aws.String(rs.Primary.Attributes[names.AttrKey]),
		})
		if err != nil {
			return err
		}
		defer output.Body.Close()
		body, err := io.ReadAll(output.Body)
		if err != nil {
			return err
		}
		if got := string(body); got != want {
			return fmt.Errorf("object content = %q, want %q", got, want)
		}
		return nil
	}
}
func testAccObjectCopyConfig_base(sourceBucket, targetBucket, sourceKey string) string {
	return fmt.Sprintf(`
locals {
  source_bucket = %[1]q
  target_bucket = %[2]q
}

resource "ctyun_oos_object" "source" {
  bucket  = local.source_bucket
  key     = %[3]q
  content = "some_bucket_content"
}
`, sourceBucket, targetBucket, sourceKey)
}

func testAccObjectCopyConfig_basic(sourceBucket, targetBucket, sourceKey, targetKey string) string {
	return acctest.ConfigCompose(testAccObjectCopyConfig_base(sourceBucket, targetBucket, sourceKey), fmt.Sprintf(`
resource "ctyun_oos_object_copy" "test" {
  bucket = local.target_bucket
  key    = %[1]q
  source = ctyun_oos_object.source.id
}
`, targetKey))
}

func testAccObjectCopyConfig_updateHeader(sourceBucket, targetBucket, sourceKey, targetKey, attribute, value string) string {
	header := ""
	if attribute != "" {
		header = fmt.Sprintf("\n  %s = %q", attribute, value)
	}

	return acctest.ConfigCompose(testAccObjectCopyConfig_base(sourceBucket, targetBucket, sourceKey), fmt.Sprintf(`
resource "ctyun_oos_object_copy" "test" {
  bucket             = local.target_bucket
  key                = %[1]q
  source             = ctyun_oos_object.source.id
  metadata_directive = "REPLACE"
%[2]s
}
`, targetKey, header))
}
func testAccObjectCopyConfig_overwriteTarget(sourceBucket, targetBucket, sourceKey, targetKey string) string {
	return acctest.ConfigCompose(testAccObjectCopyConfig_base(sourceBucket, targetBucket, sourceKey), fmt.Sprintf(`
resource "ctyun_oos_object" "existing_target" {
  bucket  = local.target_bucket
  key     = %[1]q
  content = "existing target content"
}

resource "ctyun_oos_object_copy" "test" {
  bucket = local.target_bucket
  key    = %[1]q
  source = ctyun_oos_object.source.id

  depends_on = [ctyun_oos_object.existing_target]
}
`, targetKey))
}
func testAccObjectCopyConfig_validation(bucket, key, source string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object_copy" "test" {
  bucket = %[1]q
  key    = %[2]q
  source = %[3]q
}
`, bucket, key, source)
}

func testAccObjectCopyConfig_serverSideEncryption(bucket, sourceKey, targetKey string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = %[1]q

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "ctyun_oos_object" "source" {
  bucket     = %[1]q
  key        = %[2]q
  content    = "encrypted copy content"
  depends_on = [ctyun_oos_bucket_server_side_encryption_configuration.test]
}

resource "ctyun_oos_object_copy" "test" {
  bucket     = %[1]q
  key        = %[3]q
  source     = ctyun_oos_object.source.id
  depends_on = [ctyun_oos_bucket_server_side_encryption_configuration.test]
}
`, bucket, sourceKey, targetKey)
}

func testAccObjectCopyConfig_storageClass(sourceBucket, targetBucket, sourceKey, targetKey, storageClass string) string {
	return acctest.ConfigCompose(testAccObjectCopyConfig_base(sourceBucket, targetBucket, sourceKey), fmt.Sprintf(`
resource "ctyun_oos_object_copy" "test" {
  bucket        = local.target_bucket
  key           = %[1]q
  source        = ctyun_oos_object.source.id
  storage_class = %[2]q
}
`, targetKey, storageClass))
}

func testAccObjectCopyConfig_metadataCopy(sourceBucket, targetBucket, sourceKey, targetKey string) string {
	return fmt.Sprintf(`
locals {
  source_bucket = %[1]q
  target_bucket = %[2]q
}

resource "ctyun_oos_object" "source" {
  bucket  = local.source_bucket
  key     = %[3]q
  content = "some_bucket_content"

  metadata = {
    sourcekey = "source-value"
  }
}

resource "ctyun_oos_object_copy" "test" {
  bucket             = local.target_bucket
  key                = %[4]q
  source             = ctyun_oos_object.source.id
  metadata_directive = "COPY"
}
`, sourceBucket, targetBucket, sourceKey, targetKey)
}
func testAccObjectCopyConfig_metadataEmpty(sourceBucket, targetBucket, sourceKey, targetKey string) string {
	return acctest.ConfigCompose(testAccObjectCopyConfig_base(sourceBucket, targetBucket, sourceKey), fmt.Sprintf(`
resource "ctyun_oos_object_copy" "test" {
  bucket             = local.target_bucket
  key                = %[1]q
  source             = ctyun_oos_object.source.id
  metadata_directive = "REPLACE"
}
`, targetKey))
}
func testAccObjectCopyConfig_metadataCustom(sourceBucket, targetBucket, sourceKey, targetKey, metadataKey, metadataValue string) string {
	return acctest.ConfigCompose(testAccObjectCopyConfig_base(sourceBucket, targetBucket, sourceKey), fmt.Sprintf(`
resource "ctyun_oos_object_copy" "test" {
  bucket             = local.target_bucket
  key                = %[1]q
  source             = ctyun_oos_object.source.id
  metadata_directive = "REPLACE"

  metadata = {
    %[2]s = %[3]q
  }
}
`, targetKey, metadataKey, metadataValue))
}

func testAccObjectCopyConfig_conditional(sourceBucket, targetBucket, sourceKey, targetKey, conditionField, valueExpression string) string {
	return acctest.ConfigCompose(testAccObjectCopyConfig_base(sourceBucket, targetBucket, sourceKey), fmt.Sprintf(`
resource "ctyun_oos_object_copy" "test" {
  bucket = local.target_bucket
  key    = %[1]q
  source = ctyun_oos_object.source.id
  %s     = %s
}
`, targetKey, conditionField, valueExpression))
}

func testAccObjectCopyConfig_copyIfModifiedSince(sourceBucket, targetBucket, sourceKey, targetKey, value string) string {
	return acctest.ConfigCompose(testAccObjectCopyConfig_base(sourceBucket, targetBucket, sourceKey), fmt.Sprintf(`
resource "ctyun_oos_object_copy" "test" {
  bucket                 = local.target_bucket
  key                    = %[1]q
  source                 = ctyun_oos_object.source.id
  copy_if_modified_since = %[2]q
}
`, targetKey, value))
}
