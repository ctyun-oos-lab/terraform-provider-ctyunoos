// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	tfresource "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-OOS-DSD-01 Basic List: verifies that soft-deleted objects are returned with the correct object key and soft-delete time.
// Basic list: verifies that the target soft-deleted object is present in the result.
func TestAccS3ObjectsSoftDeleteDataSource_BasicList_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "basic-list")
	key := testAccObjectsSoftDeleteKey(rName, "basic")

	testAccPutSoftDeletedObject(ctx, t, bucket, key)
	testAccDeleteSoftDeletedObject(ctx, t, bucket, key)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsSoftDeleteDataSourceConfig_basic(bucket),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(resourceName, "encoding_type", ""),
					resource.TestCheckResourceAttr(resourceName, "max_keys", "1000"),
					testAccCheckSoftDeletedObjectsContain(resourceName, key),
				),
			},
		},
	})
}

// TC-OOS-DSD-01 Full List: verifies all soft-deleted objects are returned without time filters.
func TestAccS3ObjectsSoftDeleteDataSource_FullList_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "full-list")
	keys := []string{
		testAccObjectsSoftDeleteKey(rName, "full-001"),
		testAccObjectsSoftDeleteKey(rName, "full-002"),
		testAccObjectsSoftDeleteKey(rName, "full-003"),
	}

	for _, key := range keys {
		testAccPutSoftDeletedObject(ctx, t, bucket, key)
		testAccDeleteSoftDeletedObject(ctx, t, bucket, key)
	}

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{{
			Config: testAccObjectsSoftDeleteDataSourceConfig_basic(bucket),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucket),
				resource.TestCheckResourceAttr(resourceName, "contents.#", strconv.Itoa(len(keys))),
				testAccCheckSoftDeletedObjectsContain(resourceName, keys...),
			),
		}},
	})
}

// TC-OOS-DSD-11 URL Encoding: verifies that with encoding_type=url, returned object keys are decoded back to their original values.
func TestAccS3ObjectsSoftDeleteDataSource_URLEncoding_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "url-encoding")
	key := testAccObjectsSoftDeleteKey(rName, "url/a + b")

	testAccPutSoftDeletedObject(ctx, t, bucket, key)
	testAccDeleteSoftDeletedObject(ctx, t, bucket, key)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{{
			Config: testAccObjectsSoftDeleteDataSourceConfig_encoded(bucket),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(resourceName, "encoding_type", "url"),
				testAccCheckSoftDeletedObjectsContain(resourceName, key),
			),
		}},
	})
}

// TC-OOS-DSD-02 After Only Window: verifies that only objects soft-deleted after the configured point in time are returned.
func TestAccS3ObjectsSoftDeleteDataSource_AfterOnlyWindow_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "after-only")
	oldKey := testAccObjectsSoftDeleteKey(rName, "after-old")
	newKey := testAccObjectsSoftDeleteKey(rName, "after-new")

	testAccPutSoftDeletedObject(ctx, t, bucket, oldKey)
	testAccDeleteSoftDeletedObject(ctx, t, bucket, oldKey)
	time.Sleep(2 * time.Second)
	after := time.Now().UnixMilli()
	time.Sleep(2 * time.Second)
	testAccPutSoftDeletedObject(ctx, t, bucket, newKey)
	testAccDeleteSoftDeletedObject(ctx, t, bucket, newKey)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsSoftDeleteDataSourceConfig_after(bucket, after),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "soft_delete_after", strconv.FormatInt(after, 10)),
					resource.TestCheckResourceAttr(resourceName, "contents.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "contents.0.key", newKey),
					resource.TestCheckResourceAttrSet(resourceName, "contents.0.soft_delete_at"),
					testAccCheckSoftDeletedKeysExcludes(resourceName, oldKey),
				),
			},
		},
	})
}

// TC-OOS-DSD-03 Before Only Window: verifies that only objects soft-deleted before the configured point in time are returned.
func TestAccS3ObjectsSoftDeleteDataSource_BeforeOnlyWindow_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "before-only")
	oldKey := testAccObjectsSoftDeleteKey(rName, "before-old")
	newKey := testAccObjectsSoftDeleteKey(rName, "before-new")

	testAccPutSoftDeletedObject(ctx, t, bucket, oldKey)
	testAccDeleteSoftDeletedObject(ctx, t, bucket, oldKey)
	time.Sleep(2 * time.Second)
	before := time.Now().UnixMilli()
	time.Sleep(2 * time.Second)
	testAccPutSoftDeletedObject(ctx, t, bucket, newKey)
	testAccDeleteSoftDeletedObject(ctx, t, bucket, newKey)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsSoftDeleteDataSourceConfig_before(bucket, before),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "soft_delete_before", strconv.FormatInt(before, 10)),
					resource.TestCheckResourceAttr(resourceName, "contents.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "contents.0.key", oldKey),
					resource.TestCheckResourceAttrSet(resourceName, "contents.0.soft_delete_at"),
					testAccCheckSoftDeletedKeysExcludes(resourceName, newKey),
				),
			},
		},
	})
}

// TC-OOS-DSD-04 Window Range: verifies that only objects within the soft-delete time window are returned.
func TestAccS3ObjectsSoftDeleteDataSource_WindowRange_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "window-range")
	beforeKey := testAccObjectsSoftDeleteKey(rName, "range-before")
	withinKey := testAccObjectsSoftDeleteKey(rName, "range-within")
	afterKey := testAccObjectsSoftDeleteKey(rName, "range-after")

	testAccPutSoftDeletedObject(ctx, t, bucket, beforeKey)
	testAccDeleteSoftDeletedObject(ctx, t, bucket, beforeKey)
	time.Sleep(2 * time.Second)
	after := time.Now().UnixMilli()
	time.Sleep(2 * time.Second)
	testAccPutSoftDeletedObject(ctx, t, bucket, withinKey)
	testAccDeleteSoftDeletedObject(ctx, t, bucket, withinKey)
	time.Sleep(2 * time.Second)
	before := time.Now().UnixMilli()
	time.Sleep(2 * time.Second)
	testAccPutSoftDeletedObject(ctx, t, bucket, afterKey)
	testAccDeleteSoftDeletedObject(ctx, t, bucket, afterKey)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsSoftDeleteDataSourceConfig_window(bucket, after, before, 0),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "soft_delete_after", strconv.FormatInt(after, 10)),
					resource.TestCheckResourceAttr(resourceName, "soft_delete_before", strconv.FormatInt(before, 10)),
					resource.TestCheckResourceAttr(resourceName, "contents.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "contents.0.key", withinKey),
					resource.TestCheckResourceAttrSet(resourceName, "contents.0.soft_delete_at"),
					testAccCheckSoftDeletedKeysExcludes(resourceName, beforeKey),
					testAccCheckSoftDeletedKeysExcludes(resourceName, afterKey),
				),
			},
		},
	})
}

// TC-OOS-DSD-05A MaxKeys Less Than Page Size: verifies that the result is correctly truncated when max_keys is smaller than the page size.
func TestAccS3ObjectsSoftDeleteDataSource_MaxKeysLessThanPageSize_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "maxkeys-less")
	key1 := testAccObjectsSoftDeleteKey(rName, "001")
	key2 := testAccObjectsSoftDeleteKey(rName, "002")
	key3 := testAccObjectsSoftDeleteKey(rName, "003")

	time.Sleep(2 * time.Second)
	after := time.Now().UnixMilli()
	for _, key := range []string{key1, key2, key3} {
		testAccPutSoftDeletedObject(ctx, t, bucket, key)
		testAccDeleteSoftDeletedObject(ctx, t, bucket, key)
	}
	before := time.Now().UnixMilli() + 2000

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsSoftDeleteDataSourceConfig_window(bucket, after, before, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "contents.#", "1"),
					testAccCheckSoftDeletedKeysContainsAny(resourceName, key1, key2, key3),
				),
			},
		},
	})
}

// TC-OOS-DSD-05B MaxKeys Equal Total Count: verifies that the full result is returned when max_keys equals the total object count.
func TestAccS3ObjectsSoftDeleteDataSource_MaxKeysEqualTotalCount_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "maxkeys-equal")
	keys := []string{testAccObjectsSoftDeleteKey(rName, "equal-001"), testAccObjectsSoftDeleteKey(rName, "equal-002"), testAccObjectsSoftDeleteKey(rName, "equal-003")}

	for _, key := range keys {
		testAccPutSoftDeletedObject(ctx, t, bucket, key)
		testAccDeleteSoftDeletedObject(ctx, t, bucket, key)
	}
	// The scenario bucket is unique per run, so no time window is needed; wait for the server-side soft-delete index to finish updating.
	time.Sleep(2 * time.Second)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{{
			Config: testAccObjectsSoftDeleteDataSourceConfig_maxKeys(bucket, len(keys)),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(resourceName, "contents.#", strconv.Itoa(len(keys))),
				testAccCheckSoftDeletedKeysContainsAll(resourceName, keys...),
			),
		}},
	})
}

// TC-OOS-DSD-05C MaxKeys Greater Than Page Size: verifies truncation to max_keys after cross-page aggregation.
func TestAccS3ObjectsSoftDeleteDataSource_MaxKeysGreaterThanPageSize_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "maxkeys-greater")
	const objectCount = 1002

	time.Sleep(2 * time.Second)
	after := time.Now().UnixMilli()
	for i := 0; i < objectCount; i++ {
		key := testAccObjectsSoftDeleteKey(rName, fmt.Sprintf("page-%04d", i))
		testAccPutSoftDeletedObject(ctx, t, bucket, key)
		testAccDeleteSoftDeletedObject(ctx, t, bucket, key)
	}
	before := time.Now().UnixMilli() + 2000

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{{
			Config: testAccObjectsSoftDeleteDataSourceConfig_window(bucket, after, before, 1001),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(resourceName, "contents.#", "1001"),
			),
		}},
	})
}

// TC-OOS-DSD-06 Empty Result: verifies that an empty list is returned when the time window matches no soft-deleted objects.
func TestAccS3ObjectsSoftDeleteDataSource_EmptyResult_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "empty-result")
	key := testAccObjectsSoftDeleteKey(rName, "empty")

	before := time.Now().UnixMilli()
	time.Sleep(2 * time.Second)
	testAccPutSoftDeletedObject(ctx, t, bucket, key)
	testAccDeleteSoftDeletedObject(ctx, t, bucket, key)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{{
			Config: testAccObjectsSoftDeleteDataSourceConfig_before(bucket, before),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(resourceName, "soft_delete_before", strconv.FormatInt(before, 10)),
				resource.TestCheckResourceAttr(resourceName, "contents.#", "0"),
				testAccCheckSoftDeletedKeysExcludes(resourceName, key),
			),
		}},
	})
}

// TC-OOS-DSD-07 BeforeLessThanAfter Negative: verifies that a clear error is returned when before is earlier than after.
func TestAccS3ObjectsSoftDeleteDataSource_BeforeLessThanAfter_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "before-less-after")
	after := time.Now().UnixMilli()
	before := after - 1

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{{
			Config:      testAccObjectsSoftDeleteDataSourceConfig_window(bucket, after, before, 0),
			ExpectError: regexache.MustCompile(`soft_delete_before must be greater than or equal to soft_delete_after`),
		}},
	})
}

// TC-OOS-DSD-08 Input Validation Matrix: verifies that illegal bucket names return clear errors in the schema or the read path.
func TestAccS3ObjectsSoftDeleteDataSource_InputValidationMatrix_invalidBucket(t *testing.T) {
	testCases := []struct {
		name        string
		bucket      string
		errorRegexp string
	}{
		{name: "consecutive-dots", bucket: "foo..bar", errorRegexp: `api error InvalidBucketName`},
		{name: "uppercase", bucket: "Foo.Bar", errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`},
		{name: "public-ipv4", bucket: "192.168.0.1", errorRegexp: `api error InvalidBucketName`},
		{name: "leading-dot", bucket: ".foo", errorRegexp: `api error InvalidBucketName`},
		{name: "underscore", bucket: "foo_bar", errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`},
		{name: "too-long", bucket: strings.Repeat("a", 64), errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`},
		{name: "too-short", bucket: strings.Repeat("a", 2), errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`},
		{name: "empty", bucket: "", errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testAccObjectsSoftDeleteDataSourceExpectError(t, testAccObjectsSoftDeleteDataSourceConfig_validation(testCase.bucket, ""), testCase.errorRegexp)
		})
	}
}

// TC-OOS-DSD-08 Input Validation Matrix: verifies that a non-existent bucket returns a clear error at the read stage.
func TestAccS3ObjectsSoftDeleteDataSource_InputValidationMatrix_bucketNotFound(t *testing.T) {
	bucket := acctest.RandomWithPrefix(t, "tf-test-soft-delete-missing")
	testAccObjectsSoftDeleteDataSourceExpectError(t, testAccObjectsSoftDeleteDataSourceConfig_validation(bucket, ""), `api error NoSuchBucket`)
}

// TC-OOS-DSD-08 Input Validation Matrix: verifies an invalid url encoding_type.
func TestAccS3ObjectsSoftDeleteDataSource_InputValidationMatrix_invalidEncodingType(t *testing.T) {
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(acctest.Context(t), t, "validation-encoding")
	testAccObjectsSoftDeleteDataSourceExpectError(t, testAccObjectsSoftDeleteDataSourceConfig_validation(bucket, `encoding_type = "xml"`), `expected encoding_type to be one of \["url"\], got`)
}

// TC-OOS-DSD-08 Input Validation Matrix: verifies that max_keys must be greater than or equal to 1.
func TestAccS3ObjectsSoftDeleteDataSource_InputValidationMatrix_invalidMaxKeys(t *testing.T) {
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(acctest.Context(t), t, "validation-maxkeys")
	testAccObjectsSoftDeleteDataSourceExpectError(t, testAccObjectsSoftDeleteDataSourceConfig_validation(bucket, "max_keys = 0"), `(?i)expected max_keys to be at least \(1\), got 0`)
}

// TC-OOS-DSD-08 Input Validation Matrix: verifies that encoding_type is case-insensitive.
func TestAccS3ObjectsSoftDeleteDataSource_InputValidationMatrix_encodingTypeCaseInsensitive(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "validation-encoding-case")
	key := testAccObjectsSoftDeleteKey(rName, "url-case/a + b")

	testAccPutSoftDeletedObject(ctx, t, bucket, key)
	testAccDeleteSoftDeletedObject(ctx, t, bucket, key)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{{
			Config: testAccObjectsSoftDeleteDataSourceConfig_validation(bucket, `encoding_type = "URL"`),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(resourceName, names.AttrBucket, bucket),
				resource.TestCheckResourceAttr(resourceName, "encoding_type", "URL"),
				resource.TestCheckResourceAttr(resourceName, "contents.#", "1"),
				resource.TestCheckResourceAttr(resourceName, "contents.0.key", key),
			),
		}},
	})
}

// TC-OOS-DSD-09 Read Stability: verifies that repeated refreshes of the same time window keep the result order and soft-delete times stable.
func TestAccS3ObjectsSoftDeleteDataSource_ReadStability_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	resourceName := "data.ctyun_oos_objects_soft_delete.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectsSoftDeleteScenarioBucket(ctx, t, "read-stability")
	keys := []string{testAccObjectsSoftDeleteKey(rName, "stable-001"), testAccObjectsSoftDeleteKey(rName, "stable-002")}

	// Pad the time window by 30 seconds on both sides, so that client/server clock skew or indexing delay
	// does not exclude boundary objects from the query result.
	after := time.Now().Add(-30 * time.Second).UnixMilli()
	for _, key := range keys {
		testAccPutSoftDeletedObject(ctx, t, bucket, key)
		testAccDeleteSoftDeletedObject(ctx, t, bucket, key)
	}
	time.Sleep(2 * time.Second)
	before := time.Now().Add(30 * time.Second).UnixMilli()
	config := testAccObjectsSoftDeleteDataSourceConfig_window(bucket, after, before, 0)
	var expectedContents []string
	check := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(resourceName, "contents.#", strconv.Itoa(len(keys))),
		testAccCheckSoftDeletedKeysContainsAll(resourceName, keys...),
		testAccCheckSoftDeletedContentsStable(resourceName, &expectedContents),
	)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{Config: config, Check: check},
			{Config: config, Check: check},
		},
	})
}

func testAccObjectsSoftDeleteDataSourceExpectError(t *testing.T, config, errorRegexp string) {
	t.Helper()
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{{
			Config:      config,
			ExpectError: regexache.MustCompile(errorRegexp),
		}},
	})
}

func testAccPutSoftDeletedObject(ctx context.Context, t *testing.T, bucket, key string) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
	_, err := conn.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String("text/plain"),
		Body:        strings.NewReader("soft delete me"),
	}, testAccOOSPutObjectOptions)
	if err != nil {
		t.Fatalf("putting soft-deleted object (%s/%s): %s", bucket, key, err)
	}
}

func testAccDeleteSoftDeletedObject(ctx context.Context, t *testing.T, bucket, key string) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
	_, err := conn.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("deleting soft-deleted object (%s/%s): %s", bucket, key, err)
	}

	_, err = tfresource.RetryUntilNotFound(ctx, tfs3.BucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return tfs3.FindObjectByBucketAndKey(ctx, conn, bucket, key, "", "")
	})
	if err != nil {
		t.Fatalf("waiting for soft-deleted object (%s/%s) visibility: %s", bucket, key, err)
	}
}

func testAccCheckSoftDeletedObjectsContain(resourceName string, keys ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		countRaw, ok := rs.Primary.Attributes["contents.#"]
		if !ok {
			return fmt.Errorf("attribute contents.# not found")
		}
		count, err := strconv.Atoi(countRaw)
		if err != nil {
			return fmt.Errorf("parsing contents.# %q: %w", countRaw, err)
		}

		for _, want := range keys {
			found := false
			for i := range count {
				if rs.Primary.Attributes[fmt.Sprintf("contents.%d.key", i)] != want {
					continue
				}
				if rs.Primary.Attributes[fmt.Sprintf("contents.%d.soft_delete_at", i)] == "" {
					return fmt.Errorf("soft-deleted object %q has an empty soft_delete_at", want)
				}
				found = true
				break
			}
			if !found {
				return fmt.Errorf("contents keys do not contain %q", want)
			}
		}

		return nil
	}
}

func testAccCheckSoftDeletedKeysContainsAny(resourceName string, keys ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		countRaw, ok := rs.Primary.Attributes["contents.#"]
		if !ok {
			return fmt.Errorf("attribute contents.# not found")
		}

		count, err := strconv.Atoi(countRaw)
		if err != nil {
			return fmt.Errorf("parsing contents.# %q: %w", countRaw, err)
		}

		for _, want := range keys {
			for i := range count {
				if rs.Primary.Attributes[fmt.Sprintf("contents.%d.key", i)] == want {
					return nil
				}
			}
		}

		return fmt.Errorf("contents keys do not contain any of %v", keys)
	}
}

func testAccCheckSoftDeletedContentsStable(resourceName string, expected *[]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		countRaw, ok := rs.Primary.Attributes["contents.#"]
		if !ok {
			return fmt.Errorf("attribute contents.# not found")
		}

		count, err := strconv.Atoi(countRaw)
		if err != nil {
			return fmt.Errorf("parsing contents.# %q: %w", countRaw, err)
		}

		actual := make([]string, count)
		for i := range count {
			key := rs.Primary.Attributes[fmt.Sprintf("contents.%d.key", i)]
			softDeleteAt := rs.Primary.Attributes[fmt.Sprintf("contents.%d.soft_delete_at", i)]
			if softDeleteAt == "" {
				return fmt.Errorf("contents.%d.soft_delete_at is empty", i)
			}
			actual[i] = key + "|" + softDeleteAt
		}

		if *expected == nil {
			*expected = actual
			return nil
		}
		if len(*expected) != len(actual) {
			return fmt.Errorf("contents changed from %v to %v", *expected, actual)
		}
		for i := range actual {
			if (*expected)[i] != actual[i] {
				return fmt.Errorf("contents changed from %v to %v", *expected, actual)
			}
		}

		return nil
	}
}

func testAccCheckSoftDeletedKeysContainsAll(resourceName string, keys ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		countRaw, ok := rs.Primary.Attributes["contents.#"]
		if !ok {
			return fmt.Errorf("attribute contents.# not found")
		}

		count, err := strconv.Atoi(countRaw)
		if err != nil {
			return fmt.Errorf("parsing contents.# %q: %w", countRaw, err)
		}

		for _, want := range keys {
			found := false
			for i := range count {
				if rs.Primary.Attributes[fmt.Sprintf("contents.%d.key", i)] == want {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("contents keys do not contain %q", want)
			}
		}

		return nil
	}
}

func testAccCheckSoftDeletedKeysExcludes(resourceName, key string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}

		countRaw, ok := rs.Primary.Attributes["contents.#"]
		if !ok {
			return fmt.Errorf("attribute contents.# not found")
		}

		count, err := strconv.Atoi(countRaw)
		if err != nil {
			return fmt.Errorf("parsing contents.# %q: %w", countRaw, err)
		}

		for i := range count {
			if rs.Primary.Attributes[fmt.Sprintf("contents.%d.key", i)] == key {
				return fmt.Errorf("contents keys unexpectedly contain %q", key)
			}
		}

		return nil
	}
}

func testAccObjectsSoftDeleteKey(rName, suffix string) string {
	return fmt.Sprintf("acc-soft-delete/%s/%s", rName, suffix)
}

func testAccObjectsSoftDeleteDataSourceConfig_after(bucket string, after int64) string {
	return fmt.Sprintf(`
data "ctyun_oos_objects_soft_delete" "test" {
  bucket            = %[1]q
  soft_delete_after = %[2]d
}
`, bucket, after)
}

func testAccObjectsSoftDeleteDataSourceConfig_before(bucket string, before int64) string {
	return fmt.Sprintf(`
data "ctyun_oos_objects_soft_delete" "test" {
  bucket             = %[1]q
  soft_delete_before = %[2]d
}
`, bucket, before)
}

func testAccObjectsSoftDeleteDataSourceConfig_encoded(bucket string) string {
	return fmt.Sprintf(`
data "ctyun_oos_objects_soft_delete" "test" {
  bucket        = %[1]q
  encoding_type = "url"
}
`, bucket)
}

func testAccObjectsSoftDeleteDataSourceConfig_validation(bucket, attributes string) string {
	return fmt.Sprintf(`
data "ctyun_oos_objects_soft_delete" "test" {
  bucket = %[1]q
%[2]s
}
`, bucket, attributes)
}

func testAccObjectsSoftDeleteDataSourceConfig_basic(bucket string) string {
	return fmt.Sprintf(`
data "ctyun_oos_objects_soft_delete" "test" {
  bucket = %[1]q
}
`, bucket)
}

func testAccObjectsSoftDeleteDataSourceConfig_maxKeys(bucket string, maxKeys int) string {
	return fmt.Sprintf(`
data "ctyun_oos_objects_soft_delete" "test" {
  bucket   = %[1]q
  max_keys = %[2]d
}
`, bucket, maxKeys)
}

// testAccObjectsSoftDeleteDataSourceConfig_window builds a config bounded by a
// millisecond [after, before] soft-delete window so a shared, accumulating
// bucket only returns the current test's objects. maxKeys is optional
// (0 to omit).
func testAccObjectsSoftDeleteDataSourceConfig_window(bucket string, after, before int64, maxKeys int) string {
	maxKeysConfig := ""
	if maxKeys > 0 {
		maxKeysConfig = fmt.Sprintf("  max_keys = %d\n", maxKeys)
	}

	return fmt.Sprintf(`
data "ctyun_oos_objects_soft_delete" "test" {
  bucket             = %[1]q
  soft_delete_after  = %[2]d
  soft_delete_before = %[3]d
%[4]s
}
`, bucket, after, before, maxKeysConfig)
}
