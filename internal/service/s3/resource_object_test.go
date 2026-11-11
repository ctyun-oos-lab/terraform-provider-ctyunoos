// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const testAccObjectMetadataMaxSize = 2048

// TC-S3-OBJECT-01 Basic CRUD
// TestAccS3Object_BasicCRUD_basic verifies object creation, extended response header read-back, and destruction.
func TestAccS3Object_BasicCRUD_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	var headers testAccOOSObjectHeaders
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_basic(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExistsWithHeaders(ctx, t, resourceName, &obj, &headers),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(resourceName, names.AttrKey, key),
					resource.TestCheckResourceAttr(resourceName, names.AttrStorageClass, "STANDARD"),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, fmt.Sprintf("%s/%s", bucket, key)),
					resource.TestCheckResourceAttr(resourceName, names.AttrARN, fmt.Sprintf("arn:ctyun:oos:::%s/%s", bucket, key)),
					resource.TestCheckResourceAttrSet(resourceName, "etag"),
					testAccCheckObjectReadBackExtendedState(resourceName, &obj, &headers),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateId:           fmt.Sprintf("%s/%s", bucket, key),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"forbid_overwrite"},
			},
		},
	})
}

// TC-S3-OBJECT-01 Basic CRUD
// TestAccS3Object_BasicCRUD_etagIfMatchMismatch verifies that an IfMatch read with the old ETag is rejected after the remote object is replaced.
func TestAccS3Object_BasicCRUD_etagIfMatchMismatch(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "etag-if-match-mismatch-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{{
			Config: testAccObjectConfig_content(bucket, key, "initial content"),
			Check: resource.ComposeAggregateTestCheckFunc(
				testAccCheckObjectExists(ctx, t, resourceName, &obj),
				testAccCheckObjectBody(&obj, "initial content"),
				resource.TestCheckResourceAttrSet(resourceName, "etag"),
				testAccCheckObjectETagIfMatchMismatch(ctx, t, resourceName, "replacement content"),
			),
		}},
	})
}

// TC-S3-OBJECT-12 Import And Identity
// TestAccS3Object_ImportAndIdentity_basic verifies <bucket>/<key> import and the identity-based import block.
func TestAccS3Object_ImportAndIdentity_basic(t *testing.T) {
	testAccS3ObjectIdentityBasic(t)
}

// TC-S3-OBJECT-11 Disappears
// TestAccS3Object_Disappears_remoteDeletion verifies that refresh detects a remotely deleted object and plans re-creation.
func TestAccS3Object_Disappears_remoteDeletion(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_basic(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceObject(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-S3-OBJECT-02 Content Variants
// TestAccS3Object_ContentVariants_content verifies uploading an object directly with content.
func TestAccS3Object_ContentVariants_content(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_content(bucket, key, "some_bucket_content"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					testAccCheckObjectBody(&obj, "some_bucket_content"),
				),
			},
		},
	})
}

// TC-S3-OBJECT-02 Content Variants
// TestAccS3Object_ContentVariants_source verifies uploading an object from a local source file.
func TestAccS3Object_ContentVariants_source(t *testing.T) {
	testAccS3ObjectContentVariantsSource(t)
}

// TC-S3-OBJECT-02 Content Variants
// TestAccS3Object_ContentVariants_empty verifies uploading an empty object when neither content nor source is set.
func TestAccS3Object_ContentVariants_empty(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "empty-content-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_basic(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					testAccCheckObjectBody(&obj, ""),
				),
			},
		},
	})
}

// TC-S3-OBJECT-02 Content Variants
// TestAccS3Object_ContentVariants_contentConflictsWithSource verifies that content and source are mutually exclusive.
func TestAccS3Object_ContentVariants_contentConflictsWithSource(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "content-source-conflict-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccObjectConfig_contentSourceConflict(bucket, key),
				ExpectError: regexache.MustCompile(`(?i)conflicts with`),
			},
		},
	})
}

// TC-S3-OBJECT-02 Content Variants
// TestAccS3Object_ContentVariants_sourceMissing verifies that a non-existent source file returns a clear error.
func TestAccS3Object_ContentVariants_sourceMissing(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "missing-source-key")
	source := t.TempDir() + string(os.PathSeparator) + "does-not-exist"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccObjectConfig_source(bucket, key, source),
			ExpectError: regexache.MustCompile(`(?i)opening S3 object source`),
		}},
	})
}

// TC-S3-OBJECT-03 Update Content
// TestAccS3Object_UpdateContent_basic verifies the read-back result and a stable plan after a content update.
func TestAccS3Object_UpdateContent_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_content(bucket, key, "initial content"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					testAccCheckObjectBody(&obj, "initial content"),
				),
			},
			{
				Config: testAccObjectConfig_content(bucket, key, "updated content"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					testAccCheckObjectBody(&obj, "updated content"),
				),
			},
			{
				Config:             testAccObjectConfig_content(bucket, key, "updated content"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-OBJECT-03 Update Content
// TestAccS3Object_UpdateContent_source verifies the read-back result and a stable plan after a source file update.
func TestAccS3Object_UpdateContent_source(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "source-update-key")
	initialSource := testAccTempFile(t, "initial source content")
	updatedSource := testAccTempFile(t, "updated source content")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_source(bucket, key, initialSource),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					testAccCheckObjectBody(&obj, "initial source content"),
				),
			},
			{
				Config: testAccObjectConfig_source(bucket, key, updatedSource),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					testAccCheckObjectBody(&obj, "updated source content"),
				),
			},
			{
				Config:             testAccObjectConfig_source(bucket, key, updatedSource),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-OBJECT-10 Binary Content
// TestAccS3Object_BinaryContent_source verifies that after a binary source upload the content is byte-for-byte identical and refresh causes no encoding corruption.
func TestAccS3Object_BinaryContent_source(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "binary-content-key")
	want := []byte{0x00, 0x01, 0x02, 0x7f, 0x80, 0x81, 0xfe, 0xff, 0x0a, 0x0d}
	source := testAccTempFile(t, string(want))
	check := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectExists(ctx, t, resourceName, &obj),
		testAccCheckObjectBodyBytes(&obj, want),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: testAccObjectConfig_source(bucket, key, source), Check: check},
			{RefreshState: true, Check: check},
		},
	})
}

// TC-S3-OBJECT-14 Server Side Encryption
// TestAccS3Object_ServerSideEncryption_bucketDefault verifies that the encryption algorithm is read back for objects in a bucket with default server-side encryption enabled. The bucket cannot be deleted because of soft delete.
func TestAccS3Object_ServerSideEncryption_bucketDefault(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectCopyTargetBucket(ctx, t)
	key := testAccObjectKey(rName, "server-side-encryption-key")
	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectExists(ctx, t, resourceName, &obj),
		resource.TestCheckResourceAttr(resourceName, "server_side_encryption", "AES256"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckObjectDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_sharedBucketServerSideEncryption(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					checks,
					resource.TestCheckResourceAttr("ctyun_oos_bucket_server_side_encryption_configuration.test", "rule.#", "1"),
					resource.TestCheckResourceAttr("ctyun_oos_bucket_server_side_encryption_configuration.test", "rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
				),
			},
			{
				RefreshState: true,
				Check:        checks,
			},
		},
	})
}

// TC-S3-OBJECT-14 Server Side Encryption
// TestAccS3Object_ServerSideEncryption_bucketWithoutDefault verifies that no encryption algorithm is output for objects when bucket default encryption is not enabled. The bucket cannot be deleted because of soft delete.
func TestAccS3Object_ServerSideEncryption_bucketWithoutDefault(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectCopySourceBucket(ctx, t)
	key := testAccObjectKey(rName, "without-server-side-encryption-key")
	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectExists(ctx, t, resourceName, &obj),
		resource.TestCheckResourceAttr(resourceName, "server_side_encryption", ""),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckObjectDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_sharedBucketWithoutServerSideEncryption(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					checks,
				),
			},
			{
				RefreshState: true,
				Check:        checks,
			},
		},
	})
}

// TC-S3-OBJECT-07 Metadata And StorageClass
// TestAccS3Object_MetadataAndStorageClass_basic verifies that metadata and storage_class are read back correctly after being written.
func TestAccS3Object_MetadataAndStorageClass_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "metadata-storage-class-key")

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectExists(ctx, t, resourceName, &obj),
		resource.TestCheckResourceAttr(resourceName, "metadata.%", "1"),
		resource.TestCheckResourceAttr(resourceName, "metadata.key1", "value1"),
		resource.TestCheckResourceAttr(resourceName, names.AttrStorageClass, "STANDARD"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: testAccObjectConfig_metadataAndStorageClass(bucket, key, "key1", "value1", "STANDARD"), Check: checks},
			{RefreshState: true, Check: checks},
		},
	})
}

// TC-S3-OBJECT-07 Metadata And StorageClass
// TestAccS3Object_MetadataAndStorageClass_metadataUpdate verifies that modifying metadata while keeping STANDARD triggers a re-upload and reads back stably.
func TestAccS3Object_MetadataAndStorageClass_metadataUpdate(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "metadata-update-key")
	initialConfig := testAccObjectConfig_metadataAndStorageClass(bucket, key, "key1", "value1", "STANDARD")
	updatedConfig := testAccObjectConfig_metadataAndStorageClass(bucket, key, "key1", "value2", "STANDARD")
	initialChecks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectExists(ctx, t, resourceName, &obj),
		testAccCheckObjectBody(&obj, "some_bucket_content"),
		resource.TestCheckResourceAttr(resourceName, "metadata.key1", "value1"),
		resource.TestCheckResourceAttr(resourceName, names.AttrStorageClass, "STANDARD"),
	)
	updatedChecks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectContentEventually(ctx, t, resourceName, "some_bucket_content"),
		resource.TestCheckResourceAttr(resourceName, "metadata.key1", "value2"),
		resource.TestCheckResourceAttr(resourceName, names.AttrStorageClass, "STANDARD"),
	)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: initialConfig, Check: initialChecks},
			{Config: updatedConfig, Check: updatedChecks},
			{RefreshState: true, Check: updatedChecks},
			{Config: updatedConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
		},
	})
}

// TC-S3-OBJECT-07 Metadata And StorageClass
// TestAccS3Object_MetadataAndStorageClass_metadataEmpty verifies that state stays empty when metadata is not set.
func TestAccS3Object_MetadataAndStorageClass_metadataEmpty(t *testing.T) {
	testAccObjectMetadataValid(t, "metadata-empty-key", "", "", "STANDARD", "0")
}

// TC-S3-OBJECT-07 Metadata And StorageClass
// TestAccS3Object_MetadataAndStorageClass_metadataMaxLength verifies that 2048 bytes of metadata can be uploaded and read back.
func TestAccS3Object_MetadataAndStorageClass_metadataMaxLength(t *testing.T) {
	testAccObjectMetadataValid(t, "metadata-max-length-key", "metakey", strings.Repeat("a", testAccObjectMetadataMaxSize), "STANDARD", "1")
}

// TC-S3-OBJECT-07 Metadata And StorageClass
// TestAccS3Object_MetadataAndStorageClass_metadataOverlong verifies that metadata exceeding 2048 bytes is rejected.
func TestAccS3Object_MetadataAndStorageClass_metadataOverlong(t *testing.T) {
	testAccObjectMetadataInvalid(t, "metadata-overlong-key", "metakey", strings.Repeat("a", testAccObjectMetadataMaxSize+1), `(?i)InvalidArgument:\s*The metadata size should be less than 2K`)
}

// TC-S3-OBJECT-07 Metadata And StorageClass
// TestAccS3Object_MetadataAndStorageClass_metadataKeyNonLowercase verifies that metadata keys must be lowercase.
func TestAccS3Object_MetadataAndStorageClass_metadataKeyNonLowercase(t *testing.T) {
	testAccS3ObjectMetadataKeyNonLowercase(t)
}

// TC-S3-OBJECT-07 Metadata And StorageClass
// TestAccS3Object_MetadataAndStorageClass_multipleMetadataKeys verifies that multiple metadata keys are read back correctly.
func TestAccS3Object_MetadataAndStorageClass_multipleMetadataKeys(t *testing.T) {
	testAccObjectMetadataMultipleKeysValid(t, "metadata-multiple-keys", "value1", "value2")
}

// TC-S3-OBJECT-07 Metadata And StorageClass
// TestAccS3Object_MetadataAndStorageClass_multipleMetadataKeysTotalLimit verifies that multiple metadata keys totaling 2KB are read back correctly.
func TestAccS3Object_MetadataAndStorageClass_multipleMetadataKeysTotalLimit(t *testing.T) {
	valueSize := testAccObjectMetadataMaxSize / 2
	testAccObjectMetadataMultipleKeysValid(t, "metadata-multiple-keys-total-limit", strings.Repeat("a", valueSize), strings.Repeat("b", valueSize))
}

// TC-S3-OBJECT-07 Metadata And StorageClass
// TestAccS3Object_MetadataAndStorageClass_multipleMetadataKeysOverlong verifies that the server rejects the request when multiple metadata keys exceed 2KB in total.
func TestAccS3Object_MetadataAndStorageClass_multipleMetadataKeysOverlong(t *testing.T) {
	valueSize := testAccObjectMetadataMaxSize/2 + 1
	testAccObjectMetadataMultipleKeysInvalid(t, "metadata-multiple-keys-overlong", strings.Repeat("a", valueSize), strings.Repeat("b", valueSize), `(?i)InvalidArgument:\s*The metadata size should be less than 2K`)
}

// TC-S3-OBJECT-07 Metadata And StorageClass
// TestAccS3Object_MetadataAndStorageClass_multipleMetadataKeysNonLowercase verifies rejection when any of multiple metadata keys is not lowercase.
func TestAccS3Object_MetadataAndStorageClass_multipleMetadataKeysNonLowercase(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "metadata-multiple-keys-non-lowercase")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccObjectConfig_metadataAndStorageClassMultiple(bucket, key, "STANDARD", "keyone", "value1", "KeyTwo", "value2"),
			ExpectError: regexache.MustCompile(`Metadata must be lowercase only`),
		}},
	})
}

func testAccObjectMetadataMultipleKeysValid(t *testing.T, suffix, firstValue, secondValue string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, suffix)
	config := testAccObjectConfig_metadataAndStorageClassMultiple(bucket, key, "STANDARD", "metakeyone", firstValue, "metakeytwo", secondValue)
	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectExists(ctx, t, resourceName, &obj),
		resource.TestCheckResourceAttr(resourceName, "metadata.%", "2"),
		resource.TestCheckResourceAttr(resourceName, "metadata.metakeyone", firstValue),
		resource.TestCheckResourceAttr(resourceName, "metadata.metakeytwo", secondValue),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: config, Check: checks},
			{RefreshState: true, Check: checks},
		},
	})
}

func testAccObjectMetadataMultipleKeysInvalid(t *testing.T, suffix, firstValue, secondValue, errorRegexp string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, suffix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccObjectConfig_metadataAndStorageClassMultiple(bucket, key, "STANDARD", "metakeyone", firstValue, "metakeytwo", secondValue),
			ExpectError: regexache.MustCompile(errorRegexp),
		}},
	})
}

// TC-S3-OBJECT-07 Metadata And StorageClass
// TestAccS3Object_MetadataAndStorageClass_storageClassInvalid verifies that an illegal storage_class is rejected.
func TestAccS3Object_MetadataAndStorageClass_storageClassInvalid(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "invalid-storage-class-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccObjectConfig_metadataAndStorageClass(bucket, key, "key1", "value1", "INVALID"),
			ExpectError: regexache.MustCompile(`(?i)expected.*STANDARD`),
		}},
	})
}

func testAccObjectMetadataValid(t *testing.T, suffix, metadataKey, metadataValue, storageClass, metadataCount string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, suffix)

	checks := []resource.TestCheckFunc{
		testAccCheckObjectExists(ctx, t, resourceName, &obj),
		resource.TestCheckResourceAttr(resourceName, "metadata.%", metadataCount),
		resource.TestCheckResourceAttr(resourceName, names.AttrStorageClass, storageClass),
	}
	if metadataKey != "" {
		checks = append(checks, resource.TestCheckResourceAttr(resourceName, "metadata."+metadataKey, metadataValue))
	}

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{{
			Config: testAccObjectConfig_metadataAndStorageClass(bucket, key, metadataKey, metadataValue, storageClass),
			Check:  resource.ComposeAggregateTestCheckFunc(checks...),
		}},
	})
}

func testAccObjectMetadataInvalid(t *testing.T, suffix, metadataKey, metadataValue, errorRegexp string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, suffix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccObjectConfig_metadataAndStorageClass(bucket, key, metadataKey, metadataValue, "STANDARD"),
			ExpectError: regexache.MustCompile(errorRegexp),
		}},
	})
}

func testAccS3ObjectContentVariantsSource(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "source-key")
	filePath := testAccTempFile(t, "source content")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_source(bucket, key, filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					testAccCheckObjectBody(&obj, "source content"),
				),
			},
		},
	})
}

// TC-S3-OBJECT-04 Same Key Overwrite And ForbidOverwrite
// TestAccS3Object_SameKeyOverwriteAndForbidOverwrite_overwrite verifies that a subsequently created object overwrites the same key.
func TestAccS3Object_SameKeyOverwriteAndForbidOverwrite_overwrite(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "same-key-overwrite-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_sameKeyOverwrite(bucket, key, "initial content", "updated content"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					testAccCheckObjectBody(&obj, "updated content"),
				),
			},
			{
				Config:             testAccObjectConfig_sameKeyOverwrite(bucket, key, "initial content", "updated content"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-OBJECT-04 Same Key Overwrite And ForbidOverwrite
// TestAccS3Object_SameKeyOverwriteAndForbidOverwrite_forbidOverwrite verifies that an error is returned when overwrite is forbidden.
func TestAccS3Object_SameKeyOverwriteAndForbidOverwrite_forbidOverwrite(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "forbid-overwrite-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_content(bucket, key, "initial content"),
			},
			{
				Config:      testAccObjectConfig_forbidOverwriteConflict(bucket, key, "initial content", "updated content"),
				ExpectError: regexache.MustCompile(`(?i)(forbid|overwrite|exist|precondition|match|FileAlreadyExists)`),
			},
		},
	})
}

// TC-S3-OBJECT-04 Same Key Overwrite And ForbidOverwrite
// TestAccS3Object_SameKeyOverwriteAndForbidOverwrite_forbidOverwriteNewObject verifies that forbidding overwrite does not affect the first upload of a new object.
func TestAccS3Object_SameKeyOverwriteAndForbidOverwrite_forbidOverwriteNewObject(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "forbid-overwrite-new-object-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_forbidOverwriteNewObject(bucket, key, "initial content"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					testAccCheckObjectBody(&obj, "initial content"),
					resource.TestCheckResourceAttr(resourceName, "forbid_overwrite", "true"),
				),
			},
		},
	})
}

// TC-S3-OBJECT-05 Key Validation Matrix
// TestAccS3Object_KeyValidationMatrix_specialCharacters verifies OOS/S3-compatible special-character keys.
func TestAccS3Object_KeyValidationMatrix_specialCharacters(t *testing.T) {
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	key := testAccObjectKey(rName, "special-_.!()'@&=+$,;")

	testAccObjectKeyValidationValid(t, key)
}

// TC-S3-OBJECT-05 Key Validation Matrix
// TestAccS3Object_KeyValidationMatrix_maxLength verifies the 1024-byte key boundary.
func TestAccS3Object_KeyValidationMatrix_maxLength(t *testing.T) {
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	prefix := fmt.Sprintf("acc-object/%s/", rName)
	key := prefix + strings.Repeat("a", 1024-len(prefix))

	testAccObjectKeyValidationValid(t, key)
}

// TC-S3-OBJECT-05 Key Validation Matrix
// TestAccS3Object_KeyValidationMatrix_directoryKey verifies directory-style keys.
func TestAccS3Object_KeyValidationMatrix_directoryKey(t *testing.T) {
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	key := testAccObjectKey(rName, "directory/nested/object.txt")

	testAccObjectKeyValidationValid(t, key)
}

// TC-S3-OBJECT-05 Key Validation Matrix
// TestAccS3Object_KeyValidationMatrix_emptyKey verifies that an empty key is rejected.
func TestAccS3Object_KeyValidationMatrix_emptyKey(t *testing.T) {
	testAccObjectKeyValidationInvalid(t, "", `(?i)key must not be empty`)
}

// TC-S3-OBJECT-05 Key Validation Matrix
// TestAccS3Object_KeyValidationMatrix_overlongKey verifies that a key exceeding 1024 bytes is rejected.
func TestAccS3Object_KeyValidationMatrix_overlongKey(t *testing.T) {
	testAccObjectKeyValidationInvalid(t, strings.Repeat("a", 1025), `(?i)InvalidObjectName`)
}

// TC-S3-OBJECT-06 Bucket And Read Validation
// TestAccS3Object_BucketAndReadValidation_invalidBucket verifies that all eight categories of illegal bucket inputs are rejected.
func TestAccS3Object_BucketAndReadValidation_invalidBucket(t *testing.T) {
	testCases := []struct {
		name        string
		bucket      string
		errorRegexp string
	}{
		{name: "consecutive-dots", bucket: "foo..bar", errorRegexp: `(?i)InvalidBucketName`},
		{name: "uppercase", bucket: "Foo.Bar", errorRegexp: `(?i)must contain only lowercase`},
		{name: "ipv4-address", bucket: "192.168.0.1", errorRegexp: `(?i)InvalidBucketName`},
		{name: "loopback-address", bucket: "127.0.0.1", errorRegexp: `(?i)InvalidBucketName`},
		{name: "leading-period", bucket: ".foo", errorRegexp: `(?i)InvalidBucketName`},
		{name: "trailing-period", bucket: "bar.", errorRegexp: `(?i)InvalidBucketName`},
		{name: "underscore", bucket: "foo_bar", errorRegexp: `(?i)must contain only lowercase`},
		{name: "overlong", bucket: strings.Repeat("a", 64), errorRegexp: `(?i)expected length`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testAccObjectBucketValidationInvalid(t, testCase.bucket, testCase.errorRegexp)
		})
	}
}

// TC-S3-OBJECT-06 Bucket And Read Validation
// TestAccS3Object_BucketAndReadValidation_missingBucket verifies that uploading to a non-existent bucket returns NoSuchBucket.
func TestAccS3Object_BucketAndReadValidation_missingBucket(t *testing.T) {
	bucket := strings.ToLower(acctest.RandomWithPrefix(t, "tf-acc-missing-bucket"))
	testAccObjectBucketValidationInvalid(t, bucket, `(?i)NoSuchBucket`)
}

// TC-S3-OBJECT-06 Bucket And Read Validation
// TestAccS3Object_BucketAndReadValidation_emptyBucket verifies that an empty bucket is rejected by local validation.
func TestAccS3Object_BucketAndReadValidation_emptyBucket(t *testing.T) {
	testAccObjectBucketValidationInvalid(t, "", `(?i)expected length of bucket to be in the range \(3 - 63\)`)
}

// TC-S3-OBJECT-06 Bucket And Read Validation
// TestAccS3Object_BucketAndReadValidation_missingKey verifies that reading the same key after the object is deleted remotely returns a not-found error.
func TestAccS3Object_BucketAndReadValidation_missingKey(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(acctest.RandomWithPrefix(t, acctest.ResourcePrefix), "missing-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_content(bucket, key, "created before remote deletion"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceObject(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config:      testAccObjectConfig_missingKeyDataSource(bucket, key),
				ExpectError: regexache.MustCompile(`(?i)couldn't find resource`),
			},
		},
	})
}

func testAccObjectBucketValidationInvalid(t *testing.T, bucket, errorRegexp string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	key := testAccObjectKey(acctest.RandomWithPrefix(t, acctest.ResourcePrefix), "invalid-bucket-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccObjectConfig_content(bucket, key, "invalid bucket validation"),
			ExpectError: regexache.MustCompile(errorRegexp),
		}},
	})
}
func testAccObjectKeyValidationValid(t *testing.T, key string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	bucket := acctest.OOSSharedObjectBucket(ctx, t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_basic(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckObjectExists(ctx, t, resourceName, &obj),
					resource.TestCheckResourceAttr(resourceName, names.AttrKey, key),
				),
			},
		},
	})
}

func testAccObjectKeyValidationInvalid(t *testing.T, key, errorRegexp string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	bucket := acctest.OOSSharedObjectBucket(ctx, t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccObjectConfig_basic(bucket, key),
				ExpectError: regexache.MustCompile(errorRegexp),
			},
		},
	})
}

// TC-S3-OBJECT-13 Cross Account Access Negative
// TestAccS3Object_CrossAccountAccessNegative_read verifies that objects of another account cannot be read.
func TestAccS3Object_CrossAccountAccessNegative_read(t *testing.T) {
	ctx := acctest.Context(t)
	bucket := acctest.SkipIfEnvVarNotSet(t, "CTYUNOOS_OTHER_ACCOUNT_BUCKET_NAME")
	key := acctest.SkipIfEnvVarNotSet(t, "CTYUNOOS_OTHER_ACCOUNT_OBJECT_KEY")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config:      testAccObjectConfig_crossAccountRead(bucket, key),
				ExpectError: regexache.MustCompile(`(?i)Forbidden:\s*Forbidden`),
			},
		},
	})
}

// TC-S3-OBJECT-13 Cross Account Access Negative
// TestAccS3Object_CrossAccountAccessNegative_overwrite verifies that objects of another account cannot be overwritten.
func TestAccS3Object_CrossAccountAccessNegative_overwrite(t *testing.T) {
	ctx := acctest.Context(t)
	bucket := acctest.SkipIfEnvVarNotSet(t, "CTYUNOOS_OTHER_ACCOUNT_BUCKET_NAME")
	key := acctest.SkipIfEnvVarNotSet(t, "CTYUNOOS_OTHER_ACCOUNT_OBJECT_KEY")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config:      testAccObjectConfig_crossAccountOverwrite(bucket, key),
				ExpectError: regexache.MustCompile(`(?i)AccessDenied:\s*Access Denied`),
			},
		},
	})
}

// TC-S3-OBJECT-09 Request Headers And Throttle
// TestAccS3Object_RequestHeadersAndThrottle_cacheControl verifies the empty value, explicit value, read-back, and metadata isolation of cache_control.
func TestAccS3Object_RequestHeadersAndThrottle_cacheControl(t *testing.T) {
	testAccS3ObjectRequestHeader(t, "cache_control", "max-age=3600")
}

// TC-S3-OBJECT-09 Request Headers And Throttle
// TestAccS3Object_RequestHeadersAndThrottle_contentDisposition verifies the empty value, explicit value, read-back, and metadata isolation of content_disposition.
func TestAccS3Object_RequestHeadersAndThrottle_contentDisposition(t *testing.T) {
	testAccS3ObjectRequestHeader(t, "content_disposition", `attachment; filename="test.txt"`)
}

// TC-S3-OBJECT-09 Request Headers And Throttle
// TestAccS3Object_RequestHeadersAndThrottle_contentEncoding verifies the empty value, explicit value, read-back, and metadata isolation of content_encoding.
func TestAccS3Object_RequestHeadersAndThrottle_contentEncoding(t *testing.T) {
	testAccS3ObjectRequestHeader(t, "content_encoding", "gzip")
}

// TC-S3-OBJECT-09 Request Headers And Throttle
// TestAccS3Object_RequestHeadersAndThrottle_contentMD5 verifies the empty value, explicit value, read-back, and metadata isolation of content_md5.
func TestAccS3Object_RequestHeadersAndThrottle_contentMD5(t *testing.T) {
	testAccS3ObjectRequestHeader(t, "content_md5", "OqCS5vD+Ro43ZgOq6zK1uA==")
}

// TC-S3-OBJECT-09 Request Headers And Throttle
// TestAccS3Object_RequestHeadersAndThrottle_contentTypeDefault verifies that state is back-filled with the server-side default when content_type is not set.
func TestAccS3Object_RequestHeadersAndThrottle_contentTypeDefault(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "content-type-default-key")
	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectExists(ctx, t, resourceName, &obj),
		testAccCheckObjectContentTypeReadBack(resourceName, &obj),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: testAccObjectConfig_headersEmpty(bucket, key), Check: checks},
			{RefreshState: true, Check: checks},
		},
	})
}

// TC-S3-OBJECT-09 Request Headers And Throttle
// TestAccS3Object_RequestHeadersAndThrottle_contentType verifies the empty value, explicit value, read-back, and metadata isolation of content_type.
func TestAccS3Object_RequestHeadersAndThrottle_contentType(t *testing.T) {
	testAccS3ObjectRequestHeader(t, names.AttrContentType, "text/plain")
}

// TC-S3-OBJECT-09 Request Headers And Throttle
// TestAccS3Object_RequestHeadersAndThrottle_limit verifies the empty value, explicit value, post-refresh state, and metadata isolation of limit.
func TestAccS3Object_RequestHeadersAndThrottle_limit(t *testing.T) {
	testAccS3ObjectRequestHeader(t, "limit", "rate=1024")
}

func testAccS3ObjectRequestHeader(t *testing.T, attribute, value string) {
	t.Helper()

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	var obj s3.GetObjectOutput
	resourceName := "ctyun_oos_object.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, fmt.Sprintf("headers-%s-key", strings.ReplaceAll(attribute, "_", "-")))

	checkEmpty := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectExists(ctx, t, resourceName, &obj),
		resource.TestCheckResourceAttr(resourceName, "metadata.%", "0"),
	)
	checkExplicit := resource.ComposeAggregateTestCheckFunc(
		testAccCheckObjectExists(ctx, t, resourceName, &obj),
		resource.TestCheckResourceAttr(resourceName, attribute, value),
		resource.TestCheckResourceAttr(resourceName, "metadata.%", "1"),
		resource.TestCheckResourceAttr(resourceName, "metadata.header_owner", "acceptance-test"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckObjectDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccObjectConfig_headersEmpty(bucket, key),
				Check:  checkEmpty,
			},
			{
				Config: testAccObjectConfig_header(bucket, key, attribute, value),
				Check:  checkExplicit,
			},
			{
				RefreshState: true,
				Check:        checkExplicit,
			},
			{
				Config:             testAccObjectConfig_header(bucket, key, attribute, value),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}
func testAccS3ObjectMetadataKeyNonLowercase(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "metadata-lowercase-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccObjectConfig_metadataAndStorageClass(bucket, key, "Key1", "value1", "STANDARD"),
				ExpectError: regexache.MustCompile(`Metadata must be lowercase only`),
			},
		},
	})
}

func testAccCheckObjectDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_oos_object" {
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

func testAccCheckObjectExists(ctx context.Context, t *testing.T, n string, v *s3.GetObjectOutput) resource.TestCheckFunc {
	return testAccCheckObjectExistsWithHeaders(ctx, t, n, v, nil)
}

func testAccCheckObjectExistsWithHeaders(ctx context.Context, t *testing.T, n string, v *s3.GetObjectOutput, headers *testAccOOSObjectHeaders) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not Found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		input := &s3.GetObjectInput{
			Bucket:  aws.String(rs.Primary.Attributes[names.AttrBucket]),
			Key:     aws.String(rs.Primary.Attributes[names.AttrKey]),
			IfMatch: aws.String(rs.Primary.Attributes["etag"]),
		}

		optFns := []func(*s3.Options){}
		if headers != nil {
			optFns = append(optFns, testAccCaptureOOSObjectHeadersMiddleware(headers))
		}

		output, err := conn.GetObject(ctx, input, optFns...)
		if err != nil {
			return err
		}

		*v = *output
		return nil
	}
}

type testAccOOSObjectHeaders struct {
	metadataLocation string
	dataLocation     string
}

func testAccCaptureOOSObjectHeadersMiddleware(out *testAccOOSObjectHeaders) func(*s3.Options) {
	return func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			return stack.Deserialize.Add(
				middleware.DeserializeMiddlewareFunc("TestAccCaptureOOSObjectHeaders",
					func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
						o, metadata, err := next.HandleDeserialize(ctx, in)
						if err != nil {
							return o, metadata, err
						}

						if resp, ok := o.RawResponse.(*smithyhttp.Response); ok {
							out.metadataLocation = resp.Header.Get("x-ctyun-metadata-location")
							out.dataLocation = resp.Header.Get("x-ctyun-data-location")
						}

						return o, metadata, err
					}),
				middleware.Before,
			)
		})
	}
}

// testAccOOSPutObjectOptions keeps direct SDK fixtures aligned with the
// ctyun_oos_object resource. OOS does not support optional x-amz-checksum-*
// request trailers generated by the SDK for HTTPS PutObject requests.
func testAccOOSPutObjectOptions(o *s3.Options) {
	o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
}

func testAccCheckObjectETagIfMatchMismatch(ctx context.Context, t *testing.T, resourceName, replacementContent string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		bucket := rs.Primary.Attributes[names.AttrBucket]
		key := rs.Primary.Attributes[names.AttrKey]
		oldETag := rs.Primary.Attributes["etag"]
		if oldETag == "" {
			return fmt.Errorf("%s: etag must be set before testing IfMatch", resourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		if _, err := conn.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   strings.NewReader(replacementContent),
		}, testAccOOSPutObjectOptions); err != nil {
			return fmt.Errorf("replacing S3 Object (%s) in Bucket (%s): %w", key, bucket, err)
		}

		_, err := conn.GetObject(ctx, &s3.GetObjectInput{
			Bucket:  aws.String(bucket),
			Key:     aws.String(key),
			IfMatch: aws.String(oldETag),
		})
		if err == nil {
			return fmt.Errorf("reading replaced S3 Object (%s) with stale IfMatch ETag unexpectedly succeeded", key)
		}
		if !tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusPreconditionFailed) {
			return fmt.Errorf("reading replaced S3 Object (%s) with stale IfMatch ETag: expected HTTP %d, got %w", key, http.StatusPreconditionFailed, err)
		}

		output, err := conn.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("reading replacement S3 Object (%s) ETag: %w", key, err)
		}
		rs.Primary.Attributes["etag"] = strings.Trim(aws.ToString(output.ETag), `"`)

		return nil
	}
}
func testAccCheckObjectContentEventually(ctx context.Context, t *testing.T, resourceName, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		bucket := rs.Primary.Attributes[names.AttrBucket]
		key := rs.Primary.Attributes[names.AttrKey]
		deadline := time.Now().Add(30 * time.Second)
		var lastBody string

		for {
			output, err := conn.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			if err == nil {
				body, readErr := io.ReadAll(output.Body)
				closeErr := output.Body.Close()
				if readErr != nil {
					return fmt.Errorf("reading S3 Object (%s) body: %w", key, readErr)
				}
				if closeErr != nil {
					return fmt.Errorf("closing S3 Object (%s) body: %w", key, closeErr)
				}
				lastBody = string(body)
				if lastBody == want {
					return nil
				}
			}

			if time.Now().After(deadline) {
				if err != nil {
					return fmt.Errorf("reading S3 Object (%s) while waiting for content: %w", key, err)
				}
				return fmt.Errorf("S3 Object body = %q after propagation delay, want %q", lastBody, want)
			}
			time.Sleep(time.Second)
		}
	}
}
func testAccCheckObjectBody(obj *s3.GetObjectOutput, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		body, err := io.ReadAll(obj.Body)
		if err != nil {
			return err
		}
		obj.Body.Close()

		if got := string(body); got != want {
			return fmt.Errorf("S3 Object body = %v, want %v", got, want)
		}

		return nil
	}
}

func testAccCheckObjectBodyBytes(obj *s3.GetObjectOutput, want []byte) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		body, err := io.ReadAll(obj.Body)
		if err != nil {
			return err
		}
		obj.Body.Close()

		if !bytes.Equal(body, want) {
			return fmt.Errorf("S3 Object binary body = %x, want %x", body, want)
		}

		return nil
	}
}
func testAccCheckObjectContentTypeReadBack(resourceName string, obj *s3.GetObjectOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		want := aws.ToString(obj.ContentType)
		if want == "" {
			return fmt.Errorf("remote S3 Object ContentType is empty")
		}
		if got := rs.Primary.Attributes[names.AttrContentType]; got != want {
			return fmt.Errorf("content_type = %q, want remote default %q", got, want)
		}

		return nil
	}
}
func testAccCheckObjectReadBackExtendedState(resourceName string, obj *s3.GetObjectOutput, headers *testAccOOSObjectHeaders) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		etag := strings.Trim(aws.ToString(obj.ETag), `"`)
		if got := rs.Primary.Attributes["etag"]; got != etag {
			return fmt.Errorf("etag = %q, want %q", got, etag)
		}

		if got := rs.Primary.Attributes["metadata_location"]; got != headers.metadataLocation {
			return fmt.Errorf("metadata_location = %q, want %q", got, headers.metadataLocation)
		}

		if got := rs.Primary.Attributes["data_location"]; got != headers.dataLocation {
			return fmt.Errorf("data_location = %q, want %q", got, headers.dataLocation)
		}

		return nil
	}
}

func testAccObjectKey(rName, suffix string) string {
	return fmt.Sprintf("acc-object/%s/%s", rName, suffix)
}

func testAccObjectConfig_base(bucket string) string {
	return fmt.Sprintf(`
locals {
  bucket = %[1]q
}
`, bucket)
}

func testAccObjectConfig_basic(bucket, key string) string {
	return acctest.ConfigCompose(testAccObjectConfig_base(bucket), fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket = local.bucket
  key    = %[1]q
}
`, key))
}

func testAccObjectConfig_missingKeyDataSource(bucket, key string) string {
	return fmt.Sprintf(`
data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = %[2]q
}
`, bucket, key)
}
func testAccObjectConfig_content(bucket, key, content string) string {
	return acctest.ConfigCompose(testAccObjectConfig_base(bucket), fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket  = local.bucket
  key     = %[1]q
  content = %[2]q
}
`, key, content))
}

func testAccObjectConfig_metadataAndStorageClass(bucket, key, metadataKey, metadataValue, storageClass string) string {
	metadata := ""
	if metadataKey != "" {
		metadata = fmt.Sprintf(`

  metadata = {
    %[1]s = %[2]q
  }`, metadataKey, metadataValue)
	}

	return acctest.ConfigCompose(testAccObjectConfig_base(bucket), fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket        = local.bucket
  key           = %[1]q
  content       = "some_bucket_content"
  storage_class = %[2]q
%[3]s
}
`, key, storageClass, metadata))
}

func testAccObjectConfig_metadataAndStorageClassMultiple(bucket, key, storageClass, firstKey, firstValue, secondKey, secondValue string) string {
	return acctest.ConfigCompose(testAccObjectConfig_base(bucket), fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket        = local.bucket
  key           = %[1]q
  content       = "some_bucket_content"
  storage_class = %[2]q

  metadata = {
    %[3]s = %[4]q
    %[5]s = %[6]q
  }
}
`, key, storageClass, firstKey, firstValue, secondKey, secondValue))
}

func testAccObjectConfig_sharedBucketWithoutServerSideEncryption(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket  = %[1]q
  key     = %[2]q
  content = "object without server-side encryption"
}
`, bucket, key)
}

func testAccObjectConfig_sharedBucketServerSideEncryption(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = %[1]q

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "ctyun_oos_object" "test" {
  bucket     = %[1]q
  key        = %[2]q
  content    = "server-side-encrypted object"
  depends_on = [ctyun_oos_bucket_server_side_encryption_configuration.test]
}
`, bucket, key)
}
func testAccObjectConfig_source(bucket, key, source string) string {
	return acctest.ConfigCompose(testAccObjectConfig_base(bucket), fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket = local.bucket
  key    = %[1]q
  source = %[2]q
}
`, key, source))
}

func testAccObjectConfig_contentSourceConflict(bucket, key string) string {
	return acctest.ConfigCompose(testAccObjectConfig_base(bucket), fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket  = local.bucket
  key     = %[1]q
  content = "content"
  source  = "unused-source"
}
`, key))
}
func testAccObjectConfig_sameKeyOverwrite(bucket, key, existingContent, overwriteContent string) string {
	return acctest.ConfigCompose(testAccObjectConfig_base(bucket), fmt.Sprintf(`
resource "ctyun_oos_object" "existing" {
  bucket  = local.bucket
  key     = %[1]q
  content = %[2]q
}

resource "ctyun_oos_object" "test" {
  bucket  = local.bucket
  key     = %[1]q
  content = %[3]q

  depends_on = [ctyun_oos_object.existing]
}
`, key, existingContent, overwriteContent))
}
func testAccObjectConfig_forbidOverwriteNewObject(bucket, key, content string) string {
	return acctest.ConfigCompose(testAccObjectConfig_base(bucket), fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket           = local.bucket
  key              = %[1]q
  content          = %[2]q
  forbid_overwrite = true
}
`, key, content))
}
func testAccObjectConfig_forbidOverwriteConflict(bucket, key, existingContent, conflictContent string) string {
	return acctest.ConfigCompose(testAccObjectConfig_base(bucket), fmt.Sprintf(`
resource "ctyun_oos_object" "existing" {
  bucket  = local.bucket
  key     = %[1]q
  content = %[2]q
}

resource "ctyun_oos_object" "test" {
  bucket           = local.bucket
  key              = %[1]q
  content          = %[3]q
  forbid_overwrite = true

  depends_on = [ctyun_oos_object.existing]
}
`, key, existingContent, conflictContent))
}

func testAccObjectConfig_crossAccountRead(bucket, key string) string {
	return fmt.Sprintf(`
data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = %[2]q
}
`, bucket, key)
}

func testAccObjectConfig_crossAccountOverwrite(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket  = %[1]q
  key     = %[2]q
  content = "cross-account overwrite attempt"
}
`, bucket, key)
}
func testAccObjectConfig_headersEmpty(bucket, key string) string {
	return testAccObjectConfig_content(bucket, key, "some_bucket_content")
}

func testAccObjectConfig_header(bucket, key, attribute, value string) string {
	return acctest.ConfigCompose(testAccObjectConfig_base(bucket), fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket  = local.bucket
  key     = %[1]q
  content = "some_bucket_content"

  %[2]s = %[3]q

  metadata = {
    header_owner = "acceptance-test"
  }
}
`, key, attribute, value))
}
func testAccTempFile(t *testing.T, content string) string {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "s3-object-*")
	if err != nil {
		t.Fatalf("creating temp file: %s", err)
	}

	if _, err := file.WriteString(content); err != nil {
		file.Close()
		t.Fatalf("writing temp file: %s", err)
	}

	if err := file.Close(); err != nil {
		t.Fatalf("closing temp file: %s", err)
	}

	return file.Name()
}
