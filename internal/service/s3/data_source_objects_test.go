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
)

// TC-OOS-DOS-01 Basic List verifies that the objects data source returns all objects under the configured bucket prefix without producing common_prefix.
func TestAccS3ObjectsDataSource_BasicList_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	dataSourceName := "data.ctyun_oos_objects.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsDataSourceConfig_basic(bucket, rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, "max_keys", "1000"),
					testAccCheckResourceAttrNotSet(dataSourceName, "encoding_type"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "3"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.0", testAccObjectsDataSourcePrefix(rName)+"/prefix1/sub1/0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.1", testAccObjectsDataSourcePrefix(rName)+"/prefix1/sub2/0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.2", testAccObjectsDataSourcePrefix(rName)+"/prefix2/0"),
				),
			},
		},
	})
}

// TC-OOS-DOS-02 Prefix And Delimiter verifies that when reading with prefix and delimiter combined, common_prefix is aggregated by directory and keys is empty.
func TestAccS3ObjectsDataSource_PrefixAndDelimiter_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	dataSourceName := "data.ctyun_oos_objects.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsDataSourceConfig_prefixes(bucket, rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "2"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.0", testAccObjectsDataSourcePrefix(rName)+"/prefix1/sub1/"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.1", testAccObjectsDataSourcePrefix(rName)+"/prefix1/sub2/"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "0"),
				),
			},
		},
	})
}

// TC-OOS-DSD-14 MaxKeys Delimiter verifies that max_keys limits only the number of object keys,
// not the number of common_prefix entries aggregated via delimiter.
func TestAccS3ObjectsDataSource_MaxKeysDelimiter_commonPrefixes(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	dataSourceName := "data.ctyun_oos_objects.test"
	prefix := testAccObjectsDataSourcePrefix(rName) + "/prefix1/"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsDataSourceConfig_maxKeysDelimiter(bucket, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrPrefix, prefix),
					resource.TestCheckResourceAttr(dataSourceName, "delimiter", "/"),
					resource.TestCheckResourceAttr(dataSourceName, "max_keys", "1"),
					testAccCheckResourceAttrNotSet(dataSourceName, "encoding_type"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "2"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.0", prefix+"sub1/"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.1", prefix+"sub2/"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "0"),
				),
			},
		},
	})
}

// TC-OOS-DOS-03 URL Encoding verifies that when encoding_type is url, object keys containing spaces, +, and / are stably decoded back to their original values.
func TestAccS3ObjectsDataSource_URLEncoding_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	dataSourceName := "data.ctyun_oos_objects.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsDataSourceConfig_encoded(bucket, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, "max_keys", "1000"),
					resource.TestCheckResourceAttr(dataSourceName, "encoding_type", "url"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.0", testAccObjectsDataSourcePrefix(rName)+"/prefix/a + b"),
				),
			},
		},
	})
}

// TC-OOS-DSD-13 URL Encoding Delimiter verifies that when reading with encoding_type=url combined with delimiter,
// common_prefix is URL-decoded back to the original directory prefix.
func TestAccS3ObjectsDataSource_URLEncoding_delimiter(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	dataSourceName := "data.ctyun_oos_objects.test"
	prefix := testAccObjectsDataSourcePrefix(rName) + "/prefix/"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsDataSourceConfig_encodedDelimiter(bucket, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrPrefix, prefix),
					resource.TestCheckResourceAttr(dataSourceName, "delimiter", "/"),
					resource.TestCheckResourceAttr(dataSourceName, "encoding_type", "url"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.0", prefix+"a + b/"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "0"),
				),
			},
		},
	})
}

// TC-OOS-DSD-12 URL Encoding Case Insensitive verifies that encoding_type is case-insensitive:
// object keys are still decoded to their original values when uppercase URL is used.
func TestAccS3ObjectsDataSource_URLEncoding_caseInsensitive(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	dataSourceName := "data.ctyun_oos_objects.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsDataSourceConfig_encodedCaseInsensitive(bucket, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, "encoding_type", "URL"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.0", testAccObjectsDataSourcePrefix(rName)+"/prefix/a + b"),
				),
			},
		},
	})
}

// TC-OOS-DOS-05 MaxKeys Boundary verifies that with max_keys of 1 and 5,
// the total number of returned objects does not exceed the user-declared value.
func TestAccS3ObjectsDataSource_MaxKeysBoundary_small(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	dataSourceName := "data.ctyun_oos_objects.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsDataSourceConfig_maxKeysSmall(bucket, rName, 1, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, "max_keys", "1"),
					testAccCheckResourceAttrNotSet(dataSourceName, "encoding_type"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "1"),
				),
			},
			{
				Config: testAccObjectsDataSourceConfig_maxKeysSmall(bucket, rName, 2, 5),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, "max_keys", "5"),
					testAccCheckResourceAttrNotSet(dataSourceName, "encoding_type"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "5"),
				),
			},
		},
	})
}

// TC-OOS-DOS-05 MaxKeys Boundary verifies that with max_keys of 1000, 1001, and 1002,
// the total number of objects after OOS V1 cross-page aggregation does not exceed the user-declared value.
func TestAccS3ObjectsDataSource_MaxKeysBoundary_large(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	dataSourceName := "data.ctyun_oos_objects.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsDataSourceConfig_maxKeysLarge(bucket, rName, 334, 1000),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, "max_keys", "1000"),
					testAccCheckResourceAttrNotSet(dataSourceName, "encoding_type"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "1000"),
				),
			},
			{
				Config: testAccObjectsDataSourceConfig_maxKeysLarge(bucket, rName, 334, 1001),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, "max_keys", "1001"),
					testAccCheckResourceAttrNotSet(dataSourceName, "encoding_type"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "1001"),
				),
			},
			{
				Config: testAccObjectsDataSourceConfig_maxKeysLarge(bucket, rName, 334, 1002),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, "max_keys", "1002"),
					testAccCheckResourceAttrNotSet(dataSourceName, "encoding_type"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "1002"),
				),
			},
		},
	})
}

// TC-OOS-DOS-06 Prefix Combined verifies that when reading with prefix and max_keys combined,
// only objects within the filtered range are returned, with no prefix leakage or duplicate objects.
func TestAccS3ObjectsDataSource_PrefixCombined_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	dataSourceName := "data.ctyun_oos_objects.test"
	prefix := testAccObjectsDataSourcePrefix(rName) + "/prefix1/"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsDataSourceConfig_prefixCombined(bucket, rName, 2, 2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrPrefix, prefix),
					resource.TestCheckResourceAttr(dataSourceName, "max_keys", "2"),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "2"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.0", prefix+"sub1/0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.1", prefix+"sub1/1"),
				),
			},
		},
	})
}

// TC-OOS-DOS-07 Empty Result verifies that when the prefix matches no objects,
// both keys and common_prefix are empty and the read succeeds.
func TestAccS3ObjectsDataSource_EmptyResult_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	dataSourceName := "data.ctyun_oos_objects.test"
	prefix := testAccObjectsDataSourcePrefix(rName) + "/missing/"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config: testAccObjectsDataSourceConfig_emptyResult(bucket, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrPrefix, prefix),
					resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "keys.#", "0"),
				),
			},
		},
	})
}

// TC-OOS-DOS-08 Input Validation Matrix verifies that illegal bucket names return clear errors in the local schema or the OOS read path.
func TestAccS3ObjectsDataSource_InputValidationMatrix_invalidBucket(t *testing.T) {
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
			testAccObjectsDataSourceExpectError(t, testAccObjectsDataSourceConfig_validation(testCase.bucket, ""), testCase.errorRegexp)
		})
	}
}

// TC-OOS-DOS-08 Input Validation Matrix verifies that a non-existent bucket returns a clear error at the read stage.
func TestAccS3ObjectsDataSource_InputValidationMatrix_bucketNotFound(t *testing.T) {
	bucket := acctest.RandomWithPrefix(t, "tf-test-objects-missing")
	testAccObjectsDataSourceExpectError(t, testAccObjectsDataSourceConfig_validation(bucket, ""), `https response error StatusCode: 404.*NoSuchBucket`)
}

// TC-OOS-DOS-08 Input Validation Matrix verifies that delimiter can only be set to "/".
func TestAccS3ObjectsDataSource_InputValidationMatrix_invalidDelimiter(t *testing.T) {
	bucket := acctest.OOSSharedObjectBucket(acctest.Context(t), t)
	testAccObjectsDataSourceExpectError(t, testAccObjectsDataSourceConfig_validation(bucket, `delimiter = "-"`), `(?i)expected delimiter to be one of \["/"\], got -`)
}

// TC-OOS-DOS-08 Input Validation Matrix verifies that encoding_type can only be url.
func TestAccS3ObjectsDataSource_InputValidationMatrix_invalidEncodingType(t *testing.T) {
	bucket := acctest.OOSSharedObjectBucket(acctest.Context(t), t)
	testAccObjectsDataSourceExpectError(t, testAccObjectsDataSourceConfig_validation(bucket, `encoding_type = "xml"`), `(?i)expected encoding_type to be one of.*got xml`)
}

// TC-OOS-DOS-08 Input Validation Matrix verifies that max_keys must be greater than or equal to 1.
func TestAccS3ObjectsDataSource_InputValidationMatrix_invalidMaxKeys(t *testing.T) {
	bucket := acctest.OOSSharedObjectBucket(acctest.Context(t), t)
	testAccObjectsDataSourceExpectError(t, testAccObjectsDataSourceConfig_validation(bucket, "max_keys = 0"), `(?i)expected max_keys to be at least \(1\), got 0`)
}

func testAccObjectsDataSourceExpectError(t *testing.T, config string, errorRegexp string) {
	t.Helper()
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexache.MustCompile(errorRegexp),
			},
		},
	})
}

// TC-OOS-DOS-09 Read Stability verifies that after repeated refresh/re-apply of the same list configuration,
// the order and content of keys and common_prefix remain stable, producing no perpetual diff.
func TestAccS3ObjectsDataSource_ReadStability_basic(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	bucket := acctest.OOSSharedObjectBucket(ctx, t)
	dataSourceName := "data.ctyun_oos_objects.test"

	basicCheck := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
		resource.TestCheckResourceAttr(dataSourceName, "max_keys", "1000"),
		testAccCheckResourceAttrNotSet(dataSourceName, "encoding_type"),
		resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "0"),
		resource.TestCheckResourceAttr(dataSourceName, "keys.#", "3"),
		resource.TestCheckResourceAttr(dataSourceName, "keys.0", testAccObjectsDataSourcePrefix(rName)+"/prefix1/sub1/0"),
		resource.TestCheckResourceAttr(dataSourceName, "keys.1", testAccObjectsDataSourcePrefix(rName)+"/prefix1/sub2/0"),
		resource.TestCheckResourceAttr(dataSourceName, "keys.2", testAccObjectsDataSourcePrefix(rName)+"/prefix2/0"),
	)
	prefixCheck := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, bucket),
		resource.TestCheckResourceAttr(dataSourceName, "common_prefix.#", "2"),
		resource.TestCheckResourceAttr(dataSourceName, "common_prefix.0", testAccObjectsDataSourcePrefix(rName)+"/prefix1/sub1/"),
		resource.TestCheckResourceAttr(dataSourceName, "common_prefix.1", testAccObjectsDataSourcePrefix(rName)+"/prefix1/sub2/"),
		resource.TestCheckResourceAttr(dataSourceName, "keys.#", "0"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:                acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		PreventPostDestroyRefresh: true,
		Steps: []resource.TestStep{
			{Config: testAccObjectsDataSourceConfig_basic(bucket, rName, 1), Check: basicCheck},
			{Config: testAccObjectsDataSourceConfig_basic(bucket, rName, 1), Check: basicCheck},
			{Config: testAccObjectsDataSourceConfig_prefixes(bucket, rName, 1), Check: prefixCheck},
			{Config: testAccObjectsDataSourceConfig_prefixes(bucket, rName, 1), Check: prefixCheck},
		},
	})
}
func testAccObjectsDataSourcePrefix(rName string) string {
	return fmt.Sprintf("acc-objects/%s", rName)
}

func testAccObjectsDataSourceConfig_validation(bucket, attributes string) string {
	return fmt.Sprintf(`
data "ctyun_oos_objects" "test" {
  bucket = %[1]q
%[2]s
}
`, bucket, attributes)
}
func testAccObjectsDataSourceConfig_base(bucket, rName string, n int) string {
	prefix := testAccObjectsDataSourcePrefix(rName)

	return fmt.Sprintf(`
resource "ctyun_oos_object" "test1" {
  count = %[3]d

  bucket  = %[1]q
  key     = "%[2]s/prefix1/sub1/${count.index}"
  content = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
}

resource "ctyun_oos_object" "test2" {
  count = %[3]d

  bucket  = %[1]q
  key     = "%[2]s/prefix1/sub2/${count.index}"
  content = "0123456789"
}

resource "ctyun_oos_object" "test3" {
  count = %[3]d

  bucket  = %[1]q
  key     = "%[2]s/prefix2/${count.index}"
  content = "abcdefghijklmnopqrstuvwxyz"
}
`, bucket, prefix, n)
}

func testAccObjectsDataSourceConfig_basic(bucket, rName string, n int) string {
	return acctest.ConfigCompose(testAccObjectsDataSourceConfig_base(bucket, rName, n), fmt.Sprintf(`
data "ctyun_oos_objects" "test" {
  bucket = %[1]q
  prefix = "%[2]s/"

  depends_on = [ctyun_oos_object.test1, ctyun_oos_object.test2, ctyun_oos_object.test3]
}
`, bucket, testAccObjectsDataSourcePrefix(rName)))
}

func testAccObjectsDataSourceConfig_prefixes(bucket, rName string, n int) string {
	return acctest.ConfigCompose(testAccObjectsDataSourceConfig_base(bucket, rName, n), fmt.Sprintf(`
data "ctyun_oos_objects" "test" {
  bucket    = %[1]q
  prefix    = "%[2]s/prefix1/"
  delimiter = "/"

  depends_on = [ctyun_oos_object.test1, ctyun_oos_object.test2, ctyun_oos_object.test3]
}
`, bucket, testAccObjectsDataSourcePrefix(rName)))
}

func testAccObjectsDataSourceConfig_encoded(bucket, rName string) string {
	prefix := testAccObjectsDataSourcePrefix(rName)

	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket  = %[1]q
  key     = "%[2]s/prefix/a + b"
  content = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
}

data "ctyun_oos_objects" "test" {
  bucket        = %[1]q
  prefix        = "%[2]s/"
  encoding_type = "url"

  depends_on = [ctyun_oos_object.test]
}
`, bucket, prefix)
}

func testAccObjectsDataSourceConfig_encodedDelimiter(bucket, rName string) string {
	prefix := testAccObjectsDataSourcePrefix(rName)

	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket  = %[1]q
  key     = "%[2]s/prefix/a + b/file.txt"
  content = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
}

data "ctyun_oos_objects" "test" {
  bucket        = %[1]q
  prefix        = "%[2]s/prefix/"
  delimiter     = "/"
  encoding_type = "url"

  depends_on = [ctyun_oos_object.test]
}
`, bucket, prefix)
}
func testAccObjectsDataSourceConfig_encodedCaseInsensitive(bucket, rName string) string {
	prefix := testAccObjectsDataSourcePrefix(rName)

	return fmt.Sprintf(`
resource "ctyun_oos_object" "test" {
  bucket  = %[1]q
  key     = "%[2]s/prefix/a + b"
  content = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
}

data "ctyun_oos_objects" "test" {
  bucket        = %[1]q
  prefix        = "%[2]s/"
  encoding_type = "URL"

  depends_on = [ctyun_oos_object.test]
}
`, bucket, prefix)
}
func testAccObjectsDataSourceConfig_maxKeysDelimiter(bucket, rName string) string {
	return acctest.ConfigCompose(testAccObjectsDataSourceConfig_base(bucket, rName, 1), fmt.Sprintf(`
data "ctyun_oos_objects" "test" {
  bucket    = %[1]q
  prefix    = "%[2]s/prefix1/"
  delimiter = "/"
  max_keys  = 1

  depends_on = [ctyun_oos_object.test1, ctyun_oos_object.test2, ctyun_oos_object.test3]
}
`, bucket, testAccObjectsDataSourcePrefix(rName)))
}
func testAccObjectsDataSourceConfig_maxKeysSmall(bucket, rName string, n, maxKeys int) string {
	return acctest.ConfigCompose(testAccObjectsDataSourceConfig_base(bucket, rName, n), fmt.Sprintf(`
data "ctyun_oos_objects" "test" {
  bucket   = %[1]q
  prefix   = "%[3]s/"
  max_keys = %[2]d

  depends_on = [ctyun_oos_object.test1, ctyun_oos_object.test2, ctyun_oos_object.test3]
}
`, bucket, maxKeys, testAccObjectsDataSourcePrefix(rName)))
}

func testAccObjectsDataSourceConfig_maxKeysLarge(bucket, rName string, n, maxKeys int) string {
	return acctest.ConfigCompose(testAccObjectsDataSourceConfig_base(bucket, rName, n), fmt.Sprintf(`
data "ctyun_oos_objects" "test" {
  bucket   = %[1]q
  prefix   = "%[3]s/"
  max_keys = %[2]d

  depends_on = [ctyun_oos_object.test1, ctyun_oos_object.test2, ctyun_oos_object.test3]
}
`, bucket, maxKeys, testAccObjectsDataSourcePrefix(rName)))
}

func testAccObjectsDataSourceConfig_prefixCombined(bucket, rName string, n, maxKeys int) string {
	return acctest.ConfigCompose(testAccObjectsDataSourceConfig_base(bucket, rName, n), fmt.Sprintf(`
data "ctyun_oos_objects" "test" {
  bucket   = %[1]q
  prefix   = "%[3]s/prefix1/"
  max_keys = %[2]d

  depends_on = [ctyun_oos_object.test1, ctyun_oos_object.test2, ctyun_oos_object.test3]
}
`, bucket, maxKeys, testAccObjectsDataSourcePrefix(rName)))
}
func testAccObjectsDataSourceConfig_emptyResult(bucket, rName string) string {
	return acctest.ConfigCompose(testAccObjectsDataSourceConfig_base(bucket, rName, 1), fmt.Sprintf(`
data "ctyun_oos_objects" "test" {
  bucket = %[1]q
  prefix = "%[2]s/missing/"

  depends_on = [ctyun_oos_object.test1, ctyun_oos_object.test2, ctyun_oos_object.test3]
}
`, bucket, testAccObjectsDataSourcePrefix(rName)))
}
