// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-OOS-DO-01 Basic Metadata Read - verifies that the metadata read by the object data source matches the object resource, with a normalized ID.
func TestAccS3ObjectDataSource_BasicMetadataRead_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")
	dataSourceName := "data.ctyun_oos_object.test"
	resourceName := "ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectDataSourceConfig_basic(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, fmt.Sprintf("%s/%s", bucket, key)),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrID, resourceName, names.AttrID),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrARN, fmt.Sprintf("arn:ctyun:oos:::%s/%s", bucket, key)),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrARN, resourceName, names.AttrARN),
					testAccCheckResourceAttrNotSet(dataSourceName, "body"),
					testAccCheckResourceAttrNotSet(dataSourceName, "body_base64"),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrKey, key),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrKey, resourceName, names.AttrKey),
					resource.TestCheckResourceAttr(dataSourceName, "cache_control", ""),
					resource.TestCheckResourceAttr(dataSourceName, "content_disposition", ""),
					resource.TestCheckResourceAttr(dataSourceName, "content_encoding", ""),
					resource.TestCheckResourceAttr(dataSourceName, "content_length", "11"),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrContentType, resourceName, names.AttrContentType),
					resource.TestCheckResourceAttrSet(dataSourceName, "etag"),
					resource.TestCheckResourceAttrPair(dataSourceName, "etag", resourceName, "etag"),
					resource.TestCheckResourceAttr(dataSourceName, "expiration", ""),
					resource.TestCheckResourceAttr(dataSourceName, "server_side_encryption", ""),
					resource.TestCheckResourceAttr(dataSourceName, "expires", ""),
					resource.TestCheckResourceAttrSet(dataSourceName, "last_modified"),
					resource.TestCheckResourceAttr(dataSourceName, "metadata.%", "0"),
					resource.TestCheckResourceAttrPair(dataSourceName, "data_location", resourceName, "data_location"),
					resource.TestCheckResourceAttrPair(dataSourceName, "metadata_location", resourceName, "metadata_location"),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrStorageClass, "STANDARD"),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrStorageClass, resourceName, names.AttrStorageClass),
				),
			},
		},
	})
}

// TC-OOS-DO-02 Readable Body Auto Download - verifies that the body of a directly renderable text object is downloaded automatically.
func TestAccS3ObjectDataSource_ReadableBodyAutoDownload_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")
	dataSourceName := "data.ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectDataSourceConfig_readableBody(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "content_length", "3"),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrContentType, "text/plain"),
					resource.TestCheckResourceAttr(dataSourceName, "body", "yes"),
					testAccCheckResourceAttrNotSet(dataSourceName, "body_base64"),
				),
			},
		},
	})
}

// TC-OOS-DO-03 Download Body Disabled - verifies that only object metadata is returned when body download is explicitly disabled.
func TestAccS3ObjectDataSource_DownloadBodyDisabled_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")
	dataSourceName := "data.ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectDataSourceConfig_downloadBodyDisabled(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "content_length", "3"),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrContentType, "text/plain"),
					resource.TestCheckResourceAttrSet(dataSourceName, "etag"),
					testAccCheckResourceAttrNotSet(dataSourceName, "body"),
					testAccCheckResourceAttrNotSet(dataSourceName, "body_base64"),
				),
			},
		},
	})
}

// TC-OOS-DO-04 Range Read - verifies that a range read returns the specified byte range and repeated plans show no diff. content_length is the total object length, not the range length
func TestAccS3ObjectDataSource_RangeRead_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")
	dataSourceName := "data.ctyun_oos_object.test"
	config := testAccObjectDataSourceConfig_range(bucket, key)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "range", "bytes=0-4"),
					resource.TestCheckResourceAttr(dataSourceName, "body", "Hello"),
					resource.TestCheckResourceAttr(dataSourceName, "content_length", "11"),
				),
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-OOS-DO-04 Range Read - verifies that when body download is explicit, only the content of the specified range is Base64-encoded.
func TestAccS3ObjectDataSource_RangeRead_downloadBody(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")
	dataSourceName := "data.ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectDataSourceConfig_rangeDownloadBody(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "range", "bytes=0-4"),
					resource.TestCheckResourceAttr(dataSourceName, "body_base64", "SGVsbG8="),
					testAccCheckResourceAttrNotSet(dataSourceName, "body"),
				),
			},
		},
	})
}

// TC-OOS-DO-06 Binary Body Base64 - verifies that a binary object is stably back-filled as Base64, and that non-printable body content is not written into state.
func TestAccS3ObjectDataSource_BinaryBodyBase64_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")
	dataSourceName := "data.ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectDataSourceConfig_binaryBody(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "content_length", "4"),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrContentType, "application/octet-stream"),
					resource.TestCheckResourceAttr(dataSourceName, "body_base64", "AAECAw=="),
					testAccCheckResourceAttrNotSet(dataSourceName, "body"),
				),
			},
		},
	})
}

// TC-OOS-DO-06 Binary Body Base64 - verifies that the binary object body is not downloaded automatically when download_body is unset.
func TestAccS3ObjectDataSource_BinaryBodyNoAutoDownload_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")
	dataSourceName := "data.ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectDataSourceConfig_binaryBodyNoAutoDownload(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "content_length", "4"),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrContentType, "application/octet-stream"),
					testAccCheckResourceAttrNotSet(dataSourceName, "body"),
					testAccCheckResourceAttrNotSet(dataSourceName, "body_base64"),
				),
			},
		},
	})
}

// TC-OOS-DO-06 Binary Body Base64 - verifies that explicitly downloading a directly renderable content type back-fills both the body and the Base64 form.
func TestAccS3ObjectDataSource_BinaryBodyBase64_downloadableContentType(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")
	dataSourceName := "data.ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectDataSourceConfig_downloadableBody(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrContentType, "text/plain"),
					resource.TestCheckResourceAttr(dataSourceName, "body", "yes"),
					resource.TestCheckResourceAttr(dataSourceName, "body_base64", "eWVz"),
				),
			},
		},
	})
}

// TC-OOS-DO-07 Input Validation Matrix - verifies that illegal bucket names return clear errors in the local schema or the OOS read path.
func TestAccS3ObjectDataSource_InputValidationMatrix_invalidBucketNames(t *testing.T) {
	testCases := []struct {
		name        string
		bucket      string
		errorRegexp string
	}{
		{name: "consecutive-dots", bucket: "foo..bar", errorRegexp: `api error BadRequest`},
		{name: "uppercase", bucket: "Foo.Bar", errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`},
		{name: "public-ipv4", bucket: "192.168.0.1", errorRegexp: `api error BadRequest`},
		{name: "leading-dot", bucket: ".foo", errorRegexp: `api error BadRequest`},
		{name: "underscore", bucket: "foo_bar", errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`},
		{name: "too-long", bucket: strings.Repeat("a", 64), errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`},
		{name: "too-short", bucket: strings.Repeat("a", 2), errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`},
		{name: "empty", bucket: "", errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := acctest.Context(t)

			acctest.ParallelTest(ctx, t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(ctx, t) },
				ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccObjectDataSourceConfig_validation(testCase.bucket, "test-key"),
						ExpectError: regexache.MustCompile(testCase.errorRegexp),
					},
				},
			})
		})
	}
}

// TC-OOS-DO-07 Input Validation Matrix - verifies that a non-existent bucket returns a clear error in the read path.
func TestAccS3ObjectDataSource_InputValidationMatrix_nonexistentBucket(t *testing.T) {
	ctx := acctest.Context(t)
	bucket := strings.ToLower(acctest.RandomWithPrefix(t, "tf-acc-missing-bucket"))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccObjectDataSourceConfig_validation(bucket, "test-key"),
				ExpectError: regexache.MustCompile(`reading S3 Bucket.*Object.*couldn't find resource`),
			},
		},
	})
}

// TC-OOS-DO-07 Input Validation Matrix - verifies that an empty key returns a clear error in the read path.
func TestAccS3ObjectDataSource_InputValidationMatrix_emptyKey(t *testing.T) {
	ctx := acctest.Context(t)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccObjectDataSourceConfig_validation(bucket, ""),
				ExpectError: regexache.MustCompile(`(?i)input member Key must not be empty`),
			},
		},
	})
}

// TC-OOS-DO-07 Input Validation Matrix - verifies that a non-existent key returns a clear error in the read path.
func TestAccS3ObjectDataSource_InputValidationMatrix_nonexistentKey(t *testing.T) {
	ctx := acctest.Context(t)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(acctest.RandomWithPrefix(t, acctest.ResourcePrefix), "missing-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccObjectDataSourceConfig_validation(bucket, key),
				ExpectError: regexache.MustCompile(`reading S3 Bucket.*Object.*couldn't find resource`),
			},
		},
	})
}

// TC-OOS-DO-07 Input Validation Matrix - verifies that an illegal range returns a clear error in the object read path.
func TestAccS3ObjectDataSource_InputValidationMatrix_invalidRange(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "range-key")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccObjectDataSourceConfig_invalidRange(bucket, key),
				ExpectError: regexache.MustCompile(`(?i)invalid value for range`),
			},
		},
	})
}

// TC-OOS-DO-08 Resource Consistency - verifies that the fields read back by the data source stay consistent after the object resource is updated.
func TestAccS3ObjectDataSource_ResourceConsistency_update(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")
	dataSourceName := "data.ctyun_oos_object.test"
	resourceName := "ctyun_oos_object.test"

	check := func(content, contentType, metadataValue string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrPair(dataSourceName, names.AttrARN, resourceName, names.AttrARN),
			resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
			resource.TestCheckResourceAttrPair(dataSourceName, names.AttrKey, resourceName, names.AttrKey),
			resource.TestCheckResourceAttrPair(dataSourceName, names.AttrContentType, resourceName, names.AttrContentType),
			resource.TestCheckResourceAttrPair(dataSourceName, "etag", resourceName, "etag"),
			resource.TestCheckResourceAttrPair(dataSourceName, "metadata.version", resourceName, "metadata.version"),
			resource.TestCheckResourceAttr(dataSourceName, "content_length", fmt.Sprintf("%d", len(content))),
			resource.TestCheckResourceAttr(dataSourceName, "body", content),
			resource.TestCheckResourceAttr(dataSourceName, "metadata.version", metadataValue),
			resource.TestCheckResourceAttr(dataSourceName, names.AttrContentType, contentType),
		)
	}

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectDataSourceConfig_resourceConsistency(bucket, key, "one", "text/plain", "one"),
				Check:  check("one", "text/plain", "one"),
			},
			{
				Config: testAccObjectDataSourceConfig_resourceConsistency(bucket, key, "two", "text/csv", "two"),
				Check:  check("two", "text/csv", "two"),
			},
		},
	})
}

// TC-OOS-DO-09 Output Validation - verifies that a non-default storage class is correctly back-filled.
func TestAccS3ObjectDataSource_OutputValidation_storageClass(t *testing.T) {
	t.Skip("temporarily skipped")

	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "storage-class")
	dataSourceName := "data.ctyun_oos_object.test"
	resourceName := "ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccObjectDataSourceConfig_storageClass(bucket, key),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(dataSourceName, names.AttrStorageClass, "STANDARD_IA"),
				resource.TestCheckResourceAttrPair(dataSourceName, names.AttrStorageClass, resourceName, names.AttrStorageClass),
			),
		}},
	})
}

// TC-OOS-DO-09 Output Validation - verifies the read-back of the object response headers cache_control, content_disposition, and content_encoding.
func TestAccS3ObjectDataSource_OutputValidation_responseHeaders(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "output-headers")
	dataSourceName := "data.ctyun_oos_object.test"
	resourceName := "ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccObjectDataSourceConfig_outputHeaders(bucket, key),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair(dataSourceName, "cache_control", resourceName, "cache_control"),
				resource.TestCheckResourceAttrPair(dataSourceName, "content_disposition", resourceName, "content_disposition"),
				resource.TestCheckResourceAttrPair(dataSourceName, "content_encoding", resourceName, "content_encoding"),
				resource.TestCheckResourceAttr(dataSourceName, "cache_control", "max-age=3600"),
				resource.TestCheckResourceAttr(dataSourceName, "content_disposition", `attachment; filename="test.txt"`),
				resource.TestCheckResourceAttr(dataSourceName, "content_encoding", "gzip"),
			),
		}},
	})
}

// TC-OOS-DO-09 Output Validation - verifies that expiration is read back and last_modified is set after a lifecycle is configured.
func TestAccS3ObjectDataSource_OutputValidation_expiration(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectDataSourceExpirationBucket(ctx, t)
	key := testAccObjectKey(rName, "expiration")
	dataSourceName := "data.ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccObjectDataSourceConfig_outputExpiration(bucket, key),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet(dataSourceName, "expiration"),
				resource.TestCheckResourceAttrSet(dataSourceName, "last_modified"),
			),
		}},
	})
}

// TC-OOS-DO-09 Output Validation - verifies that the bucket default server-side encryption algorithm is back-filled as AES256.
func TestAccS3ObjectDataSource_OutputValidation_serverSideEncryption(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectDataSourceEncryptionBucket(ctx, t)
	key := testAccObjectKey(rName, "encryption")
	dataSourceName := "data.ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccObjectDataSourceConfig_outputEncryption(bucket, key),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(dataSourceName, "server_side_encryption", "AES256"),
				resource.TestCheckResourceAttrSet(dataSourceName, "last_modified"),
			),
		}},
	})
}

// TC-OOS-DO-09 Output Validation - verifies that an empty value is back-filled when Expires is not configured, with last_modified stably present.
func TestAccS3ObjectDataSource_OutputValidation_expiresAndLastModified(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "expires")
	dataSourceName := "data.ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccObjectDataSourceConfig_basic(bucket, key),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(dataSourceName, "expires", ""),
				resource.TestCheckResourceAttrSet(dataSourceName, "last_modified"),
			),
		}},
	})
}

// TC-OOS-DO-01 Basic Metadata Read - verifies that object metadata, once set, can be correctly read back by the data source.
func TestAccS3ObjectDataSource_BasicMetadataRead_metadata(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	key := testAccObjectKey(rName, "test-key")
	dataSourceName := "data.ctyun_oos_object.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectDataSourceConfig_metadata(bucket, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "metadata.%", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "metadata.key1", "value1"),
				),
			},
		},
	})
}

func testAccCheckResourceAttrNotSet(name, key string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		is, err := acctest.PrimaryInstanceState(s, name)
		if err != nil {
			return err
		}
		if _, ok := is.Attributes[key]; ok {
			return fmt.Errorf("%s: Attribute %q unexpectedly set", name, key)
		}
		return nil
	}
}
func testAccObjectDataSourceConfig_basic(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket  = %[1]q
  key     = %[2]q
  content = "Hello World"
}

data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = ctyun_oos_object.test.key
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_readableBody(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket       = %[1]q
  key          = %[2]q
  content      = "yes"
  content_type = "text/plain"
}

data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = ctyun_oos_object.test.key
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_downloadBodyDisabled(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket       = %[1]q
  key          = %[2]q
  content      = "yes"
  content_type = "text/plain"
}

data "ctyun_oos_object" "test" {
  bucket        = %[1]q
  key           = ctyun_oos_object.test.key
  download_body = false
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_range(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket       = %[1]q
  key          = %[2]q
  content      = "Hello World"
  content_type = "text/plain"
}

data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = ctyun_oos_object.test.key
  range  = "bytes=0-4"
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_rangeDownloadBody(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket       = %[1]q
  key          = %[2]q
  content      = "Hello World"
  content_type = "application/octet-stream"
}

data "ctyun_oos_object" "test" {
  bucket        = %[1]q
  key           = ctyun_oos_object.test.key
  range         = "bytes=0-4"
  download_body = true
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_binaryBody(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket       = %[1]q
  key          = %[2]q
  content      = "\u0000\u0001\u0002\u0003"
  content_type = "application/octet-stream"
}

data "ctyun_oos_object" "test" {
  bucket        = %[1]q
  key           = ctyun_oos_object.test.key
  download_body = true
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_binaryBodyNoAutoDownload(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket       = %[1]q
  key          = %[2]q
  content      = "\u0000\u0001\u0002\u0003"
  content_type = "application/octet-stream"
}

data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = ctyun_oos_object.test.key
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_downloadableBody(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket       = %[1]q
  key          = %[2]q
  content      = "yes"
  content_type = "text/plain"
}

data "ctyun_oos_object" "test" {
  bucket        = %[1]q
  key           = ctyun_oos_object.test.key
  download_body = true
}
`, bucket, key)
}
func testAccObjectDataSourceConfig_validation(bucket, key string) string {
	return fmt.Sprintf(`
data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = %[2]q
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_invalidRange(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket       = %[1]q
  key          = %[2]q
  content      = "range validation"
  content_type = "text/plain"
}

data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = ctyun_oos_object.test.key
  range  = "bytes=invalid"
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_resourceConsistency(bucket, key, content, contentType, metadataValue string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket       = %[1]q
  key          = %[2]q
  content      = %[3]q
  content_type = %[4]q

  metadata = {
    version = %[5]q
  }
}

data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = ctyun_oos_object.test.key
}
`, bucket, key, content, contentType, metadataValue)
}

func testAccObjectDataSourceConfig_storageClass(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket        = %[1]q
  key           = %[2]q
  content       = "storage class"
  storage_class = "STANDARD_IA"
}

data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = ctyun_oos_object.test.key
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_outputHeaders(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket              = %[1]q
  key                 = %[2]q
  content             = "headers"
  cache_control       = "max-age=3600"
  content_disposition = "attachment; filename=\"test.txt\""
  content_encoding    = "gzip"
  content_type        = "text/plain"
}

data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = ctyun_oos_object.test.key
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_outputExpiration(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = %[1]q

  rule {
    id     = "data-source-expiration"
    prefix = ""
    status = "Enabled"

    expiration {
      days = 1
    }
  }
}

resource "ctyun_oos_object" "test" {
  bucket       = %[1]q
  key          = %[2]q
  content      = "expiration"
  content_type = "text/plain"
  depends_on   = [ctyun_oos_bucket_lifecycle_configuration.test]
}

data "ctyun_oos_object" "test" {
  bucket = ctyun_oos_object.test.bucket
  key    = ctyun_oos_object.test.key
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_outputEncryption(bucket, key string) string {
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
  bucket       = %[1]q
  key          = %[2]q
  content      = "encryption"
  content_type = "text/plain"
  depends_on   = [ctyun_oos_bucket_server_side_encryption_configuration.test]
}

data "ctyun_oos_object" "test" {
  bucket = ctyun_oos_object.test.bucket
  key    = ctyun_oos_object.test.key
}
`, bucket, key)
}

func testAccObjectDataSourceConfig_metadata(bucket, key string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket  = %[1]q
  key     = %[2]q
  content = "some_bucket_content"

  metadata = {
    key1 = "value1"
  }
}

data "ctyun_oos_object" "test" {
  bucket = %[1]q
  key    = ctyun_oos_object.test.key
}
`, bucket, key)
}
